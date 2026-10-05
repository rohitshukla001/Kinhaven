# CareCircle

CareCircle is a caregiving agent for families who look after an elderly parent.
It is a self-hosted MCP server for the **Alexa+** track of the
[Build, Ship, Shape: Amazon Developer Hackathon](https://amazonappdev2026.devpost.com/).

The server keeps the medication schedule, records doses and daily check-ins,
tells family members when a dose is missed, and gives a weekly summary.
A web simulator shows the Alexa+ voice and screen experience. The simulator
uses Amazon Bedrock as the agent and calls the CareCircle MCP server.

- MCP spec version: 2025-11-25
- Transport: Streamable HTTP
- Language: Go

> **Status:** Work in progress. See [Roadmap](#roadmap).

## Prerequisites

- Go 1.25 or later
- GNU Make

## Quick start

1. Copy the example settings:

   ```sh
   cp .env.example .env
   ```

2. Run all checks:

   ```sh
   make check
   ```

3. Start the server:

   ```sh
   make run
   ```

4. Make sure that the server operates:

   ```sh
   curl http://127.0.0.1:8080/healthz
   ```

   The server sends `{"status":"ok","name":"carecircle","version":"dev"}`.

## Configuration

The server reads these environment variables. All of them are optional.

| Variable | Default | Description |
|---|---|---|
| `CARECIRCLE_ADDR` | `127.0.0.1:8080` | Listen address. The host must be a loopback address unless `CARECIRCLE_ALLOW_PUBLIC_BIND=true`. |
| `CARECIRCLE_ALLOW_PUBLIC_BIND` | `false` | Permits a non-loopback listen address. |
| `CARECIRCLE_DATA_DIR` | `./data` | Directory for data files. |
| `CARECIRCLE_ALLOWED_ORIGINS` | `http://localhost:8090,http://127.0.0.1:8090` | Browser origins that can call the MCP endpoint. |
| `CARECIRCLE_TIMEZONE` | `UTC` | Default IANA timezone for care schedules. |
| `CARECIRCLE_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

The server stops at startup if a value is not valid. It shows all errors together.

## Make targets

| Target | Description |
|---|---|
| `make check` | Runs `fmt-check`, `vet`, `test` and `build`. |
| `make test` | Runs all tests with the race detector. |
| `make build` | Builds the binaries into `bin/`. |
| `make run` | Runs the server. |
| `make cover` | Shows the test coverage. |

## Roadmap

| # | Task | Status |
|---|---|---|
| 1 | Project scaffold, configuration and health endpoint | Done |
| 2 | Care data model and file-backed store | To do |
| 3 | Dose schedule engine | To do |
| 4 | MCP server on Streamable HTTP with read tools | To do |
| 5 | MCP write tools and adherence summary | To do |
| 6 | Missed-dose escalation with Amazon SNS | To do |
| 7 | MCP App screens, prompts and resources | To do |
| 8 | Alexa+ simulator agent with Amazon Bedrock | To do |
| 9 | Alexa+ simulator web interface | To do |
| 10 | Demo data, end-to-end test and submission documents | To do |

## License

Apache License 2.0. See [LICENSE](LICENSE).
