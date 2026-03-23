package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"dev_gateway_dns/app/api"
	"dev_gateway_dns/app/cert"
	"dev_gateway_dns/app/cli"
	"dev_gateway_dns/app/dns"
	"dev_gateway_dns/app/models"
	"dev_gateway_dns/app/modules"
	"dev_gateway_dns/app/proxy"
	"dev_gateway_dns/app/status"
)

var version = "dev"

func main() {
	cmd, err := cli.Parse(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch cmd.Name {
	case "serve":
		runServe(cmd)
	case "install":
		runInstall(cmd)
	case "uninstall":
		if err := modules.UninstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service unregistered successfully")
	case "start":
		if err := modules.StartService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service started")
	case "stop":
		if err := modules.StopService(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Service stopped")
	case "status":
		st, err := modules.ServiceStatus()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Service status: %s\n", st)
	}
}

func runServe(cmd *cli.Command) {
	// Determine DB path
	dbPath := cmd.DBPath
	if dbPath == "" {
		dbPath = modules.DefaultDBPath()
	}

	// Open database with migrations
	migFS := modules.MigrationFS{
		FS:  migrationsFS,
		Dir: "app/migrations",
	}
	db, err := modules.OpenDB(dbPath, migFS)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Load config from DB, then override with CLI options
	config, err := modules.LoadConfigFromDB(db)
	if err != nil {
		log.Printf("Warning: failed to load config from DB, using defaults: %v", err)
		config = modules.NewDefaultConfig()
	}

	applyCLIOverrides(config, cmd)

	// Save config back to DB (CLI options override saved settings)
	if err := modules.SaveConfigToDB(db, config); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	// Initialize components
	autoRecords := dns.NewAutoRecordMap()
	queryLog := dns.NewRingBuffer(config.DNSQueryHistorySize)
	upstreamMap := dns.NewUpstreamMap(config.UpstreamDNSFallback)
	if err := upstreamMap.Build(); err != nil {
		log.Printf("Warning: failed to build upstream DNS map: %v", err)
	}

	// Certificate manager
	certManager := cert.NewManager(db)
	if err := certManager.Init(); err != nil {
		log.Fatalf("Failed to initialize certificate manager: %v", err)
	}

	// Access logger
	accessLogger := modules.NewAccessLogger(db)

	// Determine listen address (use first one for binding)
	listenAddr := "0.0.0.0"
	if len(config.ListenAddresses) > 0 && config.ListenAddresses[0] != "0.0.0.0" {
		listenAddr = config.ListenAddresses[0]
	}

	// Resolve auto IP function
	resolveAutoIP := func() string {
		if listenAddr != "0.0.0.0" {
			return listenAddr
		}
		ips, err := modules.GetAllNICIPs()
		if err != nil || len(ips) == 0 {
			return "127.0.0.1"
		}
		return ips[0]
	}

	// Reverse proxy
	reverseProxy := proxy.NewReverseProxy(
		db, listenAddr, config.HTTPPort, config.HTTPSPort,
		certManager.GetCertificate, accessLogger.Log, resolveAutoIP,
	)
	if err := reverseProxy.LoadRules(); err != nil {
		log.Printf("Warning: failed to load proxy rules: %v", err)
	}

	// Forward proxy
	forwardProxy := proxy.NewForwardProxy(
		listenAddr, config.ProxyPort,
		certManager.GetCertificate, accessLogger.Log, resolveAutoIP,
	)

	// Build auto DNS records
	buildAutoRecords(db, autoRecords, config)

	// Sync forward proxy rules
	syncForwardProxyRules(db, forwardProxy)

	// DNS server
	dnsServer := dns.NewServer(db, autoRecords, upstreamMap, queryLog, listenAddr, config.DNSPort)

	// API server
	// Prepare embedded frontend filesystem
	feFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Printf("Warning: failed to load frontend assets: %v", err)
	}

	apiServer := api.NewServer(db, config, reverseProxy, forwardProxy, certManager, autoRecords, queryLog, version, feFS)

	// Start all servers
	if err := dnsServer.Start(); err != nil {
		log.Printf("Warning: DNS server failed to start: %v", err)
	}
	if err := reverseProxy.Start(); err != nil {
		log.Printf("Warning: reverse proxy failed to start: %v", err)
	}
	if err := forwardProxy.Start(); err != nil {
		log.Printf("Warning: forward proxy failed to start: %v", err)
	}
	if err := apiServer.Start(); err != nil {
		log.Printf("Warning: API server failed to start: %v", err)
	}

	// Startup health check
	log.Println("Running startup health checks...")
	results := status.RunHealthChecks(config.HTTPPort, config.HTTPSPort, config.DNSPort, config.ProxyPort, config.AdminPort)
	for _, r := range results {
		if r.Bound {
			log.Printf("  [OK] %s (:%d/%s)", r.Service, r.Port, r.Protocol)
		} else {
			log.Printf("  [WARN] %s (:%d/%s) - not responding", r.Service, r.Port, r.Protocol)
		}
	}

	log.Printf("DevGatewayDNS %s is running", version)
	log.Printf("Admin UI: http://%s:%d", listenAddr, config.AdminPort)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	apiServer.Stop()
	forwardProxy.Stop()
	reverseProxy.Stop()
	dnsServer.Stop()
	log.Println("Shutdown complete")
}

func runInstall(cmd *cli.Command) {
	// Build service arguments from CLI options
	var args []string
	args = append(args, "serve")
	if cmd.DBPath != "" {
		args = append(args, "--db", cmd.DBPath)
	}
	if cmd.Config.HTTPPort != 80 {
		args = append(args, "--http-port", fmt.Sprintf("%d", cmd.Config.HTTPPort))
	}
	if cmd.Config.HTTPSPort != 443 {
		args = append(args, "--https-port", fmt.Sprintf("%d", cmd.Config.HTTPSPort))
	}
	if cmd.Config.DNSPort != 53 {
		args = append(args, "--dns-port", fmt.Sprintf("%d", cmd.Config.DNSPort))
	}
	if cmd.Config.ProxyPort != 8888 {
		args = append(args, "--proxy-port", fmt.Sprintf("%d", cmd.Config.ProxyPort))
	}
	if cmd.Config.AdminPort != 9090 {
		args = append(args, "--admin-port", fmt.Sprintf("%d", cmd.Config.AdminPort))
	}
	for _, l := range cmd.Listens {
		args = append(args, "--listen", l)
	}

	if err := modules.InstallService(modules.ServiceConfig{
		Name:        "DevGatewayDNS",
		DisplayName: "DevGatewayDNS",
		Description: "Integrated reverse proxy, DNS server, and forward proxy for local development",
		Arguments:   args,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Service registered successfully")
}

func applyCLIOverrides(config *modules.AppConfig, cmd *cli.Command) {
	if cmd.HasFlag("http-port") {
		config.HTTPPort = cmd.Config.HTTPPort
	}
	if cmd.HasFlag("https-port") {
		config.HTTPSPort = cmd.Config.HTTPSPort
	}
	if cmd.HasFlag("dns-port") {
		config.DNSPort = cmd.Config.DNSPort
	}
	if cmd.HasFlag("proxy-port") {
		config.ProxyPort = cmd.Config.ProxyPort
	}
	if cmd.HasFlag("admin-port") {
		config.AdminPort = cmd.Config.AdminPort
	}
	if cmd.HasFlag("listen") {
		config.ListenAddresses = cmd.Listens
	}
}

func buildAutoRecords(db *sql.DB, autoRecords *dns.AutoRecordMap, config *modules.AppConfig) {
	ips, err := modules.ResolveListenIPs(config.ListenAddresses)
	if err != nil {
		log.Printf("Warning: failed to resolve listen IPs: %v", err)
		return
	}

	rows, err := db.Query("SELECT hostname FROM proxy_rules WHERE enabled = 1")
	if err != nil {
		log.Printf("Warning: failed to query proxy rules for auto records: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var hostname string
		if err := rows.Scan(&hostname); err != nil {
			continue
		}
		autoRecords.Set(hostname, ips)
		count++
	}

	log.Printf("Auto DNS records built: %d hostnames, %d listen IPs (%s)", count, len(ips), strings.Join(ips, ", "))
}

func syncForwardProxyRules(db *sql.DB, fp *proxy.ForwardProxy) {
	rows, err := db.Query(
		"SELECT id, hostname, backend_protocol, backend_ip, backend_port, enabled, created_at, updated_at FROM proxy_rules WHERE enabled = 1",
	)
	if err != nil {
		log.Printf("Warning: failed to load rules for forward proxy: %v", err)
		return
	}
	defer rows.Close()

	rules := make(map[string]*models.ProxyRule)
	for rows.Next() {
		var rule models.ProxyRule
		if err := rows.Scan(&rule.ID, &rule.Hostname, &rule.BackendProtocol, &rule.BackendIP,
			&rule.BackendPort, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			continue
		}
		rules[rule.Hostname] = &rule
	}
	fp.SetRules(rules)
}
