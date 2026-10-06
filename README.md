# Kinhaven

Kinhaven is a caregiving agent for families who look after an elderly parent.

## Overview

Families who look after an elderly parent must make sure that the parent takes
each medicine at the correct time. They must also know quickly when a dose is
missed. Kinhaven keeps the medicine schedule, records doses and daily
check-ins, and tells family members when a dose is missed.

Kinhaven is a self-hosted [Model Context Protocol](https://modelcontextprotocol.io)
(MCP) server. An AI assistant, such as an Alexa+ agent, connects to the server
and uses its tools. A web simulator shows the Alexa+ voice and screen
experience.

Kinhaven is an entry in the Alexa+ track of the
[Build, Ship, Shape: Amazon Developer Hackathon](https://amazonappdev2026.devpost.com/).

## Key features

- Care records for each person: medicines with daily or weekly dose times,
  dose events (taken or skipped), daily check-ins and family contacts.
- Local JSON data file with private permissions (`0600`) and atomic writes.
- Configuration from environment variables, with all errors shown together at startup.
- Loopback-only listen address by default, as the MCP specification recommends.
- Health endpoint at `GET /healthz`.
- Graceful shutdown on `SIGINT` and `SIGTERM`.

## Architecture

```text
Browser (Alexa+ simulator UI)
        │
        ▼
Simulator agent ──────────► Amazon Bedrock
        │
        │  MCP, Streamable HTTP (spec 2025-11-25)
        ▼
Kinhaven MCP server ────► Data store
        │
        ▼
Amazon SNS ───────────────► Family members (SMS)
```

## Prerequisites

- Go 1.25 or later
- GNU Make
- Git

## Local setup

```sh
git clone https://github.com/rohitshukla001/AmazonDeveloperHackathon.git
cd AmazonDeveloperHackathon
cp .env.example .env
make check
make run
```

To make sure that the server operates, send a request to the health endpoint:

```sh
curl http://127.0.0.1:8080/healthz
```

The server sends `{"status":"ok","name":"kinhaven","version":"dev"}`.

## Configuration

The server reads these environment variables. All of them are optional.
Do not put secrets in `.env.example`.

| Variable | Default | Purpose |
|---|---|---|
| `KINHAVEN_ADDR` | `127.0.0.1:8080` | Listen address. The host must be a loopback address unless `KINHAVEN_ALLOW_PUBLIC_BIND=true`. |
| `KINHAVEN_ALLOW_PUBLIC_BIND` | `false` | Permits a non-loopback listen address. Use it only behind a trusted proxy. |
| `KINHAVEN_DATA_DIR` | `./data` | Directory for the data file. The file contains health data, so keep the directory private. |
| `KINHAVEN_ALLOWED_ORIGINS` | `http://localhost:8090,http://127.0.0.1:8090` | Browser origins that can call the MCP endpoint. |
| `KINHAVEN_TIMEZONE` | `UTC` | Default IANA timezone for care schedules. |
| `KINHAVEN_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

## Running and testing

| Command | Purpose |
|---|---|
| `make run` | Runs the server. |
| `make test` | Runs all tests with the race detector. |
| `make lint` | Runs the `gofmt` check and `go vet`. |
| `make build` | Builds the binaries into `bin/`. |
| `make check` | Runs `lint`, `test` and `build`. |
| `make cover` | Shows the total test coverage. |

## Project structure

```text
cmd/kinhaven-server/     Server entry point
internal/care/           Care data types and their validation rules
internal/config/         Loads and validates configuration
internal/server/         HTTP handler and server lifecycle
internal/store/          Saves care data to a JSON file
internal/version/        Build metadata
```

## Troubleshooting

| Problem | Solution |
|---|---|
| `"0.0.0.0" is not a loopback host` | Use `127.0.0.1` in `KINHAVEN_ADDR`, or set `KINHAVEN_ALLOW_PUBLIC_BIND=true`. |
| `address already in use` | Another process uses the port. Set a different port in `KINHAVEN_ADDR`. |
| `unknown IANA timezone` | Use a name from the IANA database, for example `Asia/Kolkata`. |
| `go.mod requires go >= 1.25.0` | Install Go 1.25 or later. |

## Related documentation

- [MCP specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25)
- [MCP Streamable HTTP transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http)

## License

Apache License 2.0. See [LICENSE](LICENSE).
