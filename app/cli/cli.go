package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"dev_gateway_dns/app/modules"
)

// Command represents the parsed CLI command.
type Command struct {
	Name     string
	Config   *modules.AppConfig
	DBPath   string
	Listens  []string
	SetFlags map[string]bool // tracks which flags were explicitly specified
}

// HasFlag returns true if the given flag name was explicitly set on the CLI.
func (c *Command) HasFlag(name string) bool {
	return c.SetFlags[name]
}

// Parse parses command-line arguments and returns the command to execute.
func Parse(version string) (*Command, error) {
	if len(os.Args) < 2 {
		printUsage(version)
		os.Exit(0)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "version":
		fmt.Printf("DevGatewayDNS %s\n", version)
		os.Exit(0)
	case "serve", "install":
		return parseServeInstall(subcommand)
	case "uninstall", "start", "stop", "status":
		return &Command{Name: subcommand, SetFlags: make(map[string]bool)}, nil
	case "help", "--help", "-h":
		printUsage(version)
		os.Exit(0)
	default:
		return nil, fmt.Errorf("unknown command: %s", subcommand)
	}

	return nil, nil
}

func parseServeInstall(name string) (*Command, error) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)

	httpPort := fs.Int("http-port", 0, "HTTP listen port (default: 80)")
	httpsPort := fs.Int("https-port", 0, "HTTPS listen port (default: 443)")
	dnsPort := fs.Int("dns-port", 0, "DNS listen port (default: 53)")
	proxyPort := fs.Int("proxy-port", 0, "Forward proxy port (default: 8888)")
	adminPort := fs.Int("admin-port", 0, "Admin UI port (default: 9090)")
	dbPath := fs.String("db", "", "Database file path")

	var listens multiFlag
	fs.Var(&listens, "listen", "Listen address (can be specified multiple times)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil, err
	}

	// Track which flags were explicitly set
	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	cmd := &Command{
		Name:     name,
		Config:   modules.NewDefaultConfig(),
		DBPath:   *dbPath,
		Listens:  []string(listens),
		SetFlags: setFlags,
	}

	if setFlags["http-port"] {
		cmd.Config.HTTPPort = *httpPort
	}
	if setFlags["https-port"] {
		cmd.Config.HTTPSPort = *httpsPort
	}
	if setFlags["dns-port"] {
		cmd.Config.DNSPort = *dnsPort
	}
	if setFlags["proxy-port"] {
		cmd.Config.ProxyPort = *proxyPort
	}
	if setFlags["admin-port"] {
		cmd.Config.AdminPort = *adminPort
	}
	if setFlags["listen"] {
		cmd.Config.ListenAddresses = []string(listens)
	}

	return cmd, nil
}

func printUsage(version string) {
	fmt.Printf(`DevGatewayDNS %s

Usage:
  devgatewaydns <command> [options]

Commands:
  serve        Start in foreground
  install      Register as OS service
  uninstall    Unregister OS service
  start        Start registered service
  stop         Stop registered service
  status       Show service status
  version      Show version

Options (serve/install):
  --http-port <port>     HTTP listen port (default: 80)
  --https-port <port>    HTTPS listen port (default: 443)
  --dns-port <port>      DNS listen port (default: 53)
  --proxy-port <port>    Forward proxy port (default: 8888)
  --admin-port <port>    Admin UI port (default: 9090)
  --listen <addr>        Listen address (repeatable, default: 0.0.0.0)
  --db <path>            Database file path
`, version)
}

// multiFlag allows multiple -listen flags.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(val string) error {
	*m = append(*m, val)
	return nil
}
