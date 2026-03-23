package dns

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	mdns "codeberg.org/miekg/dns"
	"codeberg.org/miekg/dns/dnsutil"
	"codeberg.org/miekg/dns/rdata"
)

// Server is the DNS server handling auto records, manual records, and upstream forwarding.
type Server struct {
	db          *sql.DB
	autoRecords *AutoRecordMap
	upstream    *UpstreamMap
	queryLog    *RingBuffer
	listenAddr  string
	port        int
	udpServer   *mdns.Server
	tcpServer   *mdns.Server
}

// NewServer creates a new DNS server.
func NewServer(db *sql.DB, autoRecords *AutoRecordMap, upstream *UpstreamMap, queryLog *RingBuffer, listenAddr string, port int) *Server {
	return &Server{
		db:          db,
		autoRecords: autoRecords,
		upstream:    upstream,
		queryLog:    queryLog,
		listenAddr:  listenAddr,
		port:        port,
	}
}

// Start starts the DNS server on UDP and TCP.
func (s *Server) Start() error {
	addr := net.JoinHostPort(s.listenAddr, fmt.Sprintf("%d", s.port))

	mux := mdns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)

	s.udpServer = &mdns.Server{Addr: addr, Net: "udp", Handler: mux}
	s.tcpServer = &mdns.Server{Addr: addr, Net: "tcp", Handler: mux}

	go func() {
		if err := s.udpServer.ListenAndServe(); err != nil {
			log.Printf("DNS UDP server error: %v", err)
			s.udpServer = nil
		}
	}()
	go func() {
		if err := s.tcpServer.ListenAndServe(); err != nil {
			log.Printf("DNS TCP server error: %v", err)
			s.tcpServer = nil
		}
	}()

	log.Printf("DNS server listening on %s (UDP/TCP)", addr)
	return nil
}

// Stop shuts down the DNS server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.udpServer != nil {
		s.udpServer.Shutdown(ctx)
	}
	if s.tcpServer != nil {
		s.tcpServer.Shutdown(ctx)
	}
}

// handleQuery processes DNS queries.
// Priority: manual records (DB) > auto records (memory) > upstream forwarding.
// When a hostname exists in both manual and auto, manual takes precedence.
func (s *Server) handleQuery(_ context.Context, w mdns.ResponseWriter, r *mdns.Msg) {
	start := time.Now()
	msg := r.Copy()
	dnsutil.SetReply(msg, r)
	msg.Authoritative = true

	if len(r.Question) == 0 {
		s.writeResponse(w, msg)
		return
	}

	q := r.Question[0]
	hostname := strings.TrimSuffix(q.Header().Name, ".")
	qtype := mdns.RRToType(q)
	clientIP := extractClientIP(w.RemoteAddr())
	typeName := dnsutil.TypeToString(qtype)

	// For A records: manual > auto > upstream
	// For all other types: manual > upstream (auto records are A-type only)
	if qtype == mdns.TypeA {
		if rrs := s.lookupManualRecord(hostname, "A"); len(rrs) > 0 {
			msg.Answer = append(msg.Answer, rrs...)
			s.writeResponse(w, msg)
			s.logQuery(clientIP, hostname, typeName, "manual", time.Since(start))
			return
		}
		if ips, ok := s.autoRecords.Lookup(hostname); ok {
			for _, ip := range ips {
				addr, err := netip.ParseAddr(ip)
				if err != nil {
					continue
				}
				msg.Answer = append(msg.Answer, &mdns.A{
					Hdr: mdns.Header{Name: q.Header().Name, TTL: 60, Class: mdns.ClassINET},
					A:   rdata.A{Addr: addr},
				})
			}
			s.writeResponse(w, msg)
			s.logQuery(clientIP, hostname, typeName, "auto", time.Since(start))
			return
		}
	} else {
		typeStr := qtypeToDBType(qtype)
		if typeStr != "" {
			if rrs := s.lookupManualRecord(hostname, typeStr); len(rrs) > 0 {
				msg.Answer = append(msg.Answer, rrs...)
				s.writeResponse(w, msg)
				s.logQuery(clientIP, hostname, typeName, "manual", time.Since(start))
				return
			}
		}
	}

	// Upstream forwarding
	s.forwardQuery(w, r, clientIP)
	s.logQuery(clientIP, hostname, typeName, "upstream", time.Since(start))
}

// qtypeToDBType maps DNS query types to database record type strings.
func qtypeToDBType(qtype uint16) string {
	switch qtype {
	case mdns.TypeA:
		return "A"
	case mdns.TypeAAAA:
		return "AAAA"
	case mdns.TypeCNAME:
		return "CNAME"
	case mdns.TypeMX:
		return "MX"
	case mdns.TypeTXT:
		return "TXT"
	case mdns.TypeSRV:
		return "SRV"
	case mdns.TypeNS:
		return "NS"
	case mdns.TypePTR:
		return "PTR"
	case mdns.TypeCAA:
		return "CAA"
	case mdns.TypeSOA:
		return "SOA"
	case mdns.TypeNAPTR:
		return "NAPTR"
	case mdns.TypeSSHFP:
		return "SSHFP"
	case mdns.TypeTLSA:
		return "TLSA"
	case mdns.TypeDS:
		return "DS"
	case mdns.TypeDNSKEY:
		return "DNSKEY"
	default:
		return ""
	}
}

func (s *Server) lookupManualRecord(hostname, recordType string) []mdns.RR {
	rows, err := s.db.Query(
		"SELECT name, type, value, ttl, priority, weight, port FROM dns_records WHERE name = ? AND type = ?",
		hostname, recordType,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var rrs []mdns.RR
	for rows.Next() {
		var name, rtype, value string
		var ttl int
		var priority, weight, port *int
		if err := rows.Scan(&name, &rtype, &value, &ttl, &priority, &weight, &port); err != nil {
			continue
		}
		fqdn := name + "."
		hdr := mdns.Header{Name: fqdn, TTL: uint32(ttl), Class: mdns.ClassINET}

		switch rtype {
		case "A":
			addr, err := netip.ParseAddr(value)
			if err != nil {
				continue
			}
			rrs = append(rrs, &mdns.A{Hdr: hdr, A: rdata.A{Addr: addr}})

		case "AAAA":
			addr, err := netip.ParseAddr(value)
			if err != nil {
				continue
			}
			rrs = append(rrs, &mdns.AAAA{Hdr: hdr, AAAA: rdata.AAAA{Addr: addr}})

		case "CNAME":
			rrs = append(rrs, &mdns.CNAME{Hdr: hdr, CNAME: rdata.CNAME{Target: value + "."}})

		case "MX":
			prio := uint16(10)
			if priority != nil {
				prio = uint16(*priority)
			}
			rrs = append(rrs, &mdns.MX{Hdr: hdr, MX: rdata.MX{Preference: prio, Mx: value + "."}})

		case "TXT":
			rrs = append(rrs, &mdns.TXT{Hdr: hdr, TXT: rdata.TXT{Txt: []string{value}}})

		case "SRV":
			p := uint16(0)
			w := uint16(0)
			pt := uint16(0)
			if priority != nil {
				p = uint16(*priority)
			}
			if weight != nil {
				w = uint16(*weight)
			}
			if port != nil {
				pt = uint16(*port)
			}
			rrs = append(rrs, &mdns.SRV{Hdr: hdr, SRV: rdata.SRV{Priority: p, Weight: w, Port: pt, Target: value + "."}})

		case "NS":
			rrs = append(rrs, &mdns.NS{Hdr: hdr, NS: rdata.NS{Ns: value + "."}})

		case "PTR":
			rrs = append(rrs, &mdns.PTR{Hdr: hdr, PTR: rdata.PTR{Ptr: value + "."}})

		case "CAA":
			// value format: "flag tag value", e.g. "0 issue letsencrypt.org"
			parts := strings.SplitN(value, " ", 3)
			if len(parts) < 3 {
				continue
			}
			flag, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			rrs = append(rrs, &mdns.CAA{Hdr: hdr, CAA: rdata.CAA{Flag: uint8(flag), Tag: parts[1], Value: parts[2]}})

		case "SOA":
			// value format: "ns rname serial refresh retry expire minimum"
			parts := strings.Fields(value)
			if len(parts) < 7 {
				continue
			}
			serial, _ := strconv.ParseUint(parts[2], 10, 32)
			refresh, _ := strconv.ParseUint(parts[3], 10, 32)
			retry, _ := strconv.ParseUint(parts[4], 10, 32)
			expire, _ := strconv.ParseUint(parts[5], 10, 32)
			minimum, _ := strconv.ParseUint(parts[6], 10, 32)
			rrs = append(rrs, &mdns.SOA{Hdr: hdr, SOA: rdata.SOA{
				Ns:      parts[0] + ".",
				Mbox:    parts[1] + ".",
				Serial:  uint32(serial),
				Refresh: uint32(refresh),
				Retry:   uint32(retry),
				Expire:  uint32(expire),
				Minttl:  uint32(minimum),
			}})

		case "NAPTR":
			// value format: "order preference flags service regexp replacement"
			parts := strings.SplitN(value, " ", 6)
			if len(parts) < 6 {
				continue
			}
			order, _ := strconv.ParseUint(parts[0], 10, 16)
			pref, _ := strconv.ParseUint(parts[1], 10, 16)
			rrs = append(rrs, &mdns.NAPTR{Hdr: hdr, NAPTR: rdata.NAPTR{
				Order:       uint16(order),
				Preference:  uint16(pref),
				Flags:       parts[2],
				Service:     parts[3],
				Regexp:      parts[4],
				Replacement: parts[5] + ".",
			}})

		case "SSHFP":
			// value format: "algorithm fptype fingerprint"
			parts := strings.Fields(value)
			if len(parts) < 3 {
				continue
			}
			algo, _ := strconv.Atoi(parts[0])
			fpType, _ := strconv.Atoi(parts[1])
			rrs = append(rrs, &mdns.SSHFP{Hdr: hdr, SSHFP: rdata.SSHFP{
				Algorithm:   uint8(algo),
				Type:        uint8(fpType),
				FingerPrint: parts[2],
			}})

		case "TLSA":
			// value format: "usage selector matchingtype certificate"
			parts := strings.Fields(value)
			if len(parts) < 4 {
				continue
			}
			usage, _ := strconv.Atoi(parts[0])
			selector, _ := strconv.Atoi(parts[1])
			matchType, _ := strconv.Atoi(parts[2])
			rrs = append(rrs, &mdns.TLSA{Hdr: hdr, TLSA: rdata.TLSA{
				Usage:        uint8(usage),
				Selector:     uint8(selector),
				MatchingType: uint8(matchType),
				Certificate:  parts[3],
			}})

		case "DS":
			// value format: "keytag algorithm digesttype digest"
			parts := strings.Fields(value)
			if len(parts) < 4 {
				continue
			}
			keyTag, _ := strconv.ParseUint(parts[0], 10, 16)
			algo, _ := strconv.Atoi(parts[1])
			digestType, _ := strconv.Atoi(parts[2])
			rrs = append(rrs, &mdns.DS{Hdr: hdr, DS: rdata.DS{
				KeyTag:     uint16(keyTag),
				Algorithm:  uint8(algo),
				DigestType: uint8(digestType),
				Digest:     parts[3],
			}})

		case "DNSKEY":
			// value format: "flags protocol algorithm publickey"
			parts := strings.Fields(value)
			if len(parts) < 4 {
				continue
			}
			flags, _ := strconv.ParseUint(parts[0], 10, 16)
			protocol, _ := strconv.Atoi(parts[1])
			algo, _ := strconv.Atoi(parts[2])
			rrs = append(rrs, &mdns.DNSKEY{Hdr: hdr, DNSKEY: rdata.DNSKEY{
				Flags:     uint16(flags),
				Protocol:  uint8(protocol),
				Algorithm: uint8(algo),
				PublicKey: parts[3],
			}})
		}
	}
	return rrs
}

func (s *Server) forwardQuery(w mdns.ResponseWriter, r *mdns.Msg, clientIP string) {
	upstreams := s.upstream.Resolve(clientIP)
	c := &mdns.Client{}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, upstream := range upstreams {
		addr := upstream
		if !strings.Contains(addr, ":") {
			addr = net.JoinHostPort(addr, "53")
		}
		resp, _, err := c.Exchange(ctx, r, "udp", addr)
		if err == nil && resp != nil {
			s.writeResponse(w, resp)
			return
		}
	}

	// All upstreams failed; return SERVFAIL
	msg := r.Copy()
	dnsutil.SetReply(msg, r)
	msg.Rcode = mdns.RcodeServerFailure
	s.writeResponse(w, msg)
}

func (s *Server) writeResponse(w mdns.ResponseWriter, msg *mdns.Msg) {
	if err := msg.Pack(); err != nil {
		log.Printf("DNS pack error: %v", err)
		return
	}
	io.Copy(w, msg)
}

func (s *Server) logQuery(clientIP, hostname, recordType, responseType string, duration time.Duration) {
	s.queryLog.Add(QueryLogEntry{
		ClientIP:     clientIP,
		Hostname:     hostname,
		RecordType:   recordType,
		ResponseType: responseType,
		ResponseTime: duration,
		Timestamp:    time.Now(),
	})
}

func extractClientIP(addr net.Addr) string {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP.String()
	case *net.TCPAddr:
		return a.IP.String()
	default:
		host, _, _ := net.SplitHostPort(addr.String())
		return host
	}
}
