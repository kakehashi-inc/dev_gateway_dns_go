# DevGatewayDNS

[Japanese / 日本語](README-ja.md)

## 1. System Overview

DevGatewayDNS is an integrated development tool that enables all clients on a local network, including smartphones connected via WiFi, to access virtual web pages by hostname.

Key features:

- **Reverse Proxy**: Routes HTTP/HTTPS requests to backend services based on hostname. Supports SNI-based routing, automatic header/cookie handling.
- **DNS Server**: Automatically generates A records linked to proxy rules, manual record management, and per-NIC upstream DNS forwarding.
- **Forward Proxy**: Provides an HTTP proxy for clients that cannot change DNS settings (e.g., iOS devices).
- **SSL Certificate Management**: Automatic self-signed CA certificate generation, per-host certificate issuance, and QR code distribution for mobile devices.
- **Web UI**: Manage proxy settings, DNS records, certificates, status monitoring, and system settings from a browser. Supports Japanese and English.
- **REST API / WebSocket**: Full-featured API for the admin UI with real-time log streaming.
- **OS Service Registration**: Can be registered and managed as a Windows/macOS/Linux service.
- **Single Binary Distribution**: Frontend assets, migration SQL, etc. are all embedded in the binary. Supports 6 platforms.

Tech stack: Go, SQLite (WAL), codeberg.org/miekg/dns, kardianos/service, nhooyr.io/websocket, pressly/goose v3

## 2. Developer Reference

### Development Rules

- Developer documentation (except `README.md`) must be placed in the `Documents` directory.
- Always run the linter after changes and apply appropriate fixes. If intentionally allowing a linter error, document the reason in a comment. **Builds are for releases only; running the linter is sufficient for debugging.**
- When implementing models, place one file per table.
- Reusable components must be implemented as separate files in the `modules` directory.
- Temporary scripts (e.g., investigation scripts) must be placed in the `scripts` directory.
- When creating or modifying models, update `Documents/テーブル定義.md`. Table definitions must be expressed as a table per database table, showing column names, types, and relations within the table.
- When system behavior changes, update `Documents/システム仕様.md`.

### Go Commands

Install/update debug module

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

Add a module

```bash
go get <package-name>
```

Add and build a module

```bash
go install <package-name>
```

Initialize module file

```bash
go mod init <module-name>
```

Download modules (omit module name to download all from go.mod)

```bash
go mod download <module-name>
```

Tidy modules (sync source and go.mod)

```bash
go mod tidy
```

Update all modules

```bash
go get -u
```

Update Go version

```bash
go mod tidy --go=1.25
```

Clear cache

```bash
go clean --cache --testcache
```

### Build and Release

Build

```bash
go build
```

Release

```bash
make
```
