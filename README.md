# Claude Identity Injector v2

A privacy-layer plugin for EasyCLIProxyAPI that injects identity context into
Claude API requests. Designed for deployments where the upstream proxy requires
a stable, identifiable caller identity while preserving the end-user's privacy
boundary.

## Architecture

The plugin is a Go shared library (DLL) loaded by CPA's plugin runtime. It
registers four interception hooks and a management API:

- **request.intercept_after** — rewrites the upstream request body with a
  configurable system prompt and identity metadata (user_id, device_id,
  session_id). Only fires when a matching rule is found.
- **response.intercept_after** / **response.intercept_stream_chunk** — repairs
  streaming tool-call field names (dryRun → dry_run) to match client-side
  schema expectations.
- **management.register** / **management.handle** — exposes a Chinese-language
  web dashboard and a JSON status endpoint via the CPA management API.

Matching is rule-based: each rule specifies provider name patterns, requested
model globs, and upstream model globs. The first matching rule wins. When no
rule matches, strict-mode (default) silently passes the request through without
modification.

## Build

Prerequisites:
- Go 1.26+
- Zig 0.14.x (for CGO cross-compilation; no system GCC required)
- `gopkg.in/yaml.v3` (fetched automatically)

```powershell
$env:CGO_ENABLED = "1"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CC = "path\to\zig.exe cc"
go build -buildmode=c-shared -o claude-identity-injector_v2.dll .
```

The output is a DLL and a companion `.h` header. Deploy the DLL to CPA's plugin
directory (`plugins\windows\amd64\`).

## Configuration

The plugin is configured via CPA's `config.yaml` under
`plugins.configs.claude-identity-injector_v2`:

```yaml
enabled: true
priority: 100
active: true
strict_mode: true
provider: "claude"
clear_user_agent: true
rules:
  - id: default
    enabled: true
    providers:
      - "claude"
    requested_models:
      - "claude-*"
    upstream_models:
      - "claude-*"
```

All settings are hot-reloadable through the CPA management API or the web
dashboard (accessible from the Management Center plugin menu).

## Web Dashboard

The plugin ships a self-contained Chinese-language management page:

- **Status card** — plugin ID, version, uptime, active/strict/provider/UA
  state
- **Counters card** — real-time request metrics (seen, matched, injected,
  unmatched, errors, repaired tools), auto-refreshed every 5 seconds
- **Configuration panel** — toggle enabled/active/strict_mode/clear_user_agent,
  set provider
- **Rule editor** — view, add, and delete rules; change enabled state per-rule
  and save the full rule table

The dashboard reads the management key from Management Center's localStorage
(`enc::v1::` obfuscation). If the key is absent (cross-origin iframe, or
"remember password" unchecked), the page degrades to read-only mode.

## License

GNU General Public License v3.0 — see [LICENSE](LICENSE).

Copyright (C) 2026 Adkimsm