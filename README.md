# GoProject — Standard Package Boilerplate

A production-ready Go service skeleton using **only the standard library**
for HTTP and a single MSSQL driver for the database.

## Layout

```
goproject/
├── main.go                        # Entry point
├── go.mod / go.sum
├── Makefile
├── toml/                          # All config (loaded at startup)
│   ├── app.toml
│   ├── database.toml
│   └── routes.toml
├── logs/                          # Runtime log files (created automatically)
│   └── app.log
└── internal/
    ├── tomlloader/   tomlloader.go  # Reads & parses every *.toml
    ├── util/         logger.go      # File+stdout logger, helpers
    ├── integration/  mssql.go       # MSSQL pool (medium-level config)
    ├── middleware/   middleware.go   # RequestID, Logger, Recovery, APIKey
    ├── router/       router.go      # net/http ServeMux, resolves handler names
    └── apigateway/   gateway.go     # Assembles chain, graceful shutdown
```

## Quick start

```bash
# 1 — fill in database credentials in toml/database.toml
# 2 — build & run
make build
./bin/goproject

# or just
make run
```

## Config files

| File | Purpose |
|------|---------|
| `toml/app.toml` | App name, server host/port, log file path |
| `toml/database.toml` | MSSQL connection settings |
| `toml/routes.toml` | Route → handler mapping |

## Logging

Every startup writes to `logs/app.log` (path configurable in `app.toml`).
Output goes to both the file and stdout simultaneously.

## Adding a route

1. Add an entry to `toml/routes.toml`.
2. Add the handler name to the `resolve()` switch in `internal/router/router.go`.
3. Implement the handler function in the same file (or a new file in the package).
