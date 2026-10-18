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
- Daily dose schedule in each person's own timezone. Each dose is upcoming,
  due, taken, skipped or missed. Weekly medicines and daylight-saving changes
  are supported.
- Local JSON data file with private permissions (`0600`) and atomic writes.
- MCP endpoint at `/mcp` on the Streamable HTTP transport. It accepts
  protocol version 2025-11-25 and later and runs stateless, so any request
  can go to any server instance.
- MCP tools to set up care, log doses and daily check-ins, and get a
  multi-day adherence summary for family updates.
- An MCP App card for hosts that support MCP Apps. It shows the day's doses
  or the adherence summary in an Echo Show style layout, and a due or missed
  dose can be marked as taken from the card.
- Spoken one-line summaries in tool results for voice-only hosts.
- Prompts for a daily briefing and a weekly family update, and a care-plan
  resource for each person.
- Alexa+ simulator: a web page styled as an Echo Show with voice input and
  spoken replies. It hosts the MCP App card and shows every MCP tool call.
- Simulator agent: an Amazon Bedrock tool-use loop that acts as an MCP client
  to the Kinhaven server. A built-in mock model runs the same loop without
  AWS, so the project can be tested at no cost.
- Missed-dose SMS alerts to family through Amazon SNS. Each contact gets one
  message per missed dose, and alerts that fail to send are retried.
- Origin checks and DNS-rebinding protection on the MCP endpoint.
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
- For SMS alerts: an AWS account with permission to call `sns:Publish`
- For the Bedrock agent: access to an Amazon Bedrock model that supports tool use

## Local setup

```sh
git clone https://github.com/rohitshukla001/Kinhaven.git
cd Kinhaven
cp .env.example .env
make check
make seed
make run
```

The Makefile reads the variables in `.env`. The `make seed` command writes a
week of demo data into `KINHAVEN_DATA_DIR`. To start with an empty data store,
do not run `make seed`.

To make sure that the server operates, send a request to the health endpoint:

```sh
curl http://127.0.0.1:8080/healthz
```

The server sends `{"status":"ok","name":"kinhaven","version":"dev"}`.

## Demo data

The `kinhaven-seed` command adds one person, Kamala, with this data:

- Two family contacts. Priya gets missed-dose alerts. Arjun does not.
- Five medicines with a daily or weekly schedule.
- Dose records for the last seven days. Most doses are taken. Three doses are
  missed and two doses are skipped with a note.
- One check-in for each of the last seven days, with a mood and a note.
- Records of the alerts that Priya got for the three missed doses.
- Today's doses up to now. The most recent missed dose stays open, so the
  watcher sends an alert when the server starts.

All times are relative to the time when you run the command, in
`KINHAVEN_TIMEZONE`.

Run `make seed` before you start the server. The server keeps the data in
memory, and it writes over the file at the next change. The command does not
change a data store that already has people in it.

To seed the data again:

1. Stop the server.
2. Remove `data/kinhaven.json`.
3. Run `make seed`.

## Using the MCP server

Start the server, then connect any MCP client to `http://127.0.0.1:8080/mcp`.

| Tool | Purpose |
|---|---|
| `list_people` | Lists the people in care, with their IDs and timezones. |
| `list_medications` | Lists a person's medicines, dose times and weekly days. |
| `get_schedule` | Shows each dose for one day with its status: `upcoming`, `due`, `taken`, `skipped` or `missed`. |
| `get_adherence_summary` | Summarizes doses taken, skipped and missed, the adherence percentage and the check-ins for the last 1 to 90 days. |
| `add_person` | Adds a person to care for. |
| `add_contact` | Adds a family member who gets missed-dose alerts. |
| `add_medication` | Adds a medicine with daily dose times and, for weekly medicines, the days. |
| `stop_medication` | Removes a medicine from the schedule. |
| `log_dose` | Records a dose as taken or skipped. Without `scheduled_at`, it uses today's unrecorded dose that is closest to now. |
| `record_checkin` | Records how the person feels today: `good`, `okay` or `poor`, with a note. |

The `recipient_id` argument is optional when only one person is in care.

`get_schedule` and `get_adherence_summary` link to the MCP App card
`ui://kinhaven/card.html` (`text/html;profile=mcp-app`) through
`_meta.ui.resourceUri`. Hosts without MCP Apps support use the text result.

| Prompt or resource | Purpose |
|---|---|
| `daily_briefing` prompt | A short spoken briefing for the person in care. |
| `weekly_family_update` prompt | A seven-day update for the family. |
| `kinhaven://people/{recipient_id}/care-plan` | The person's medicines, dose times and family contacts as Markdown. |

To try the tools in a browser, use the MCP Inspector:

```sh
npx @modelcontextprotocol/inspector
```

Select the **Streamable HTTP** transport and enter `http://127.0.0.1:8080/mcp`.

## Using the simulator

The simulator connects to the MCP server, lists its tools and answers requests
through an agent.

1. Start the server in one terminal with `make run`.
2. Start the simulator in a second terminal with `make sim`.
3. Open `http://127.0.0.1:8090` in Chrome or Edge.
4. Press the microphone button and speak, type a request, or select a
   suggestion such as "Has Mom taken her tablets?".

The home screen shows today's doses for the person: how many are taken, and
which doses are missed, due or next. The simulator reads this with the
`get_schedule` tool.

The screen shows the MCP App card for the schedule and the weekly summary. To
record a dose from the card:

1. Select **Mark taken**.
2. Select **Tap to confirm** within 4 seconds.

The **MCP activity** panel shows each tool call from the home screen, the agent
and the card. To stop the spoken replies, set the **Spoken replies** switch to
off.

For a text-only session, run `make sim-console`.

The default model is `mock`, a rule-based planner that needs no AWS account.
To use Amazon Bedrock, set a model ID that supports tool use:

```sh
export AWS_REGION=ap-south-1
export KINHAVEN_SIM_MODEL=global.anthropic.claude-haiku-4-5-20251001-v1:0
make sim
```

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
| `KINHAVEN_MISSED_DOSE_GRACE` | `1h` | Time after a scheduled dose before the dose counts as missed. Use a Go duration, for example `45m`. |
| `KINHAVEN_ALERTS` | `log` | `sns` sends SMS through Amazon SNS. `log` writes alerts to the server log. `off` turns alerts off. |
| `KINHAVEN_ALERT_INTERVAL` | `1m` | How often the server checks for missed doses. The minimum is `1s`. |
| `KINHAVEN_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

The simulator reads these variables:

| Variable | Default | Purpose |
|---|---|---|
| `KINHAVEN_SIM_ADDR` | `127.0.0.1:8090` | Listen address of the web simulator. The same loopback rule and `KINHAVEN_ALLOW_PUBLIC_BIND` apply. |
| `KINHAVEN_SIM_MCP_URL` | `http://127.0.0.1:8080/mcp` | MCP endpoint of the Kinhaven server. |
| `KINHAVEN_SIM_MODEL` | `mock` | `mock`, or an Amazon Bedrock model ID or inference profile ID. |

### Amazon Bedrock

The simulator calls the Bedrock Converse API with the standard AWS settings.
Give the AWS identity `bedrock:InvokeModel` on the model or inference profile:

```json
{
  "Version": "2012-10-17",
  "Statement": [{ "Effect": "Allow", "Action": "bedrock:InvokeModel", "Resource": "*" }]
}
```

Global inference profiles, such as `global.anthropic.claude-haiku-4-5-20251001-v1:0`,
route requests from `ap-south-1` to other AWS Regions.

### Amazon SNS

With `KINHAVEN_ALERTS=sns`, the server reads AWS settings from the standard
AWS sources: environment variables, `~/.aws/config` and `~/.aws/credentials`,
or an IAM role.

1. Set the region, for example `export AWS_REGION=ap-south-1`.
2. Give the AWS identity this permission:

   ```json
   {
     "Version": "2012-10-17",
     "Statement": [{ "Effect": "Allow", "Action": "sns:Publish", "Resource": "*" }]
   }
   ```

3. If the account is in the SNS SMS sandbox, verify each family phone number
   in the Amazon SNS console before you test.

## Running and testing

| Command | Purpose |
|---|---|
| `make seed` | Writes the demo data into an empty data store. |
| `make run` | Runs the server. |
| `make sim` | Runs the web simulator. |
| `make sim-console` | Runs the simulator in the terminal. |
| `make test` | Runs all tests with the race detector. |
| `make lint` | Runs the `gofmt` check and `go vet`. |
| `make build` | Builds the binaries into `bin/`. |
| `make check` | Runs `lint`, `test` and `build`. |
| `make cover` | Shows the total test coverage. |
| `make e2e` | Runs the end-to-end test. |

The end-to-end test builds the binaries and seeds a temporary data store. Then
it starts the server and the web simulator with the mock model. It checks these
items:

- The health endpoint, the MCP `initialize` request and the `Origin` check.
- A console conversation that reads the schedule and the summary, logs a dose
  and records a check-in.
- A web simulator request, and the MCP App card resource.
- The missed-dose alert in the server log, after 08:00 in `KINHAVEN_TIMEZONE`.

The test needs `curl` and uses ports 18080 and 18090. To use different ports,
set `E2E_MCP_PORT` and `E2E_SIM_PORT`.

## Project structure

```text
cmd/kinhaven-server/     Server entry point
cmd/kinhaven-sim/        Simulator entry point (web and console)
cmd/kinhaven-seed/       Demo data command
internal/agent/          Agent loop, Amazon Bedrock model and mock model
internal/sim/            Simulator web API and MCP Apps host page
internal/alert/          Missed-dose watcher and SMS senders
internal/care/           Care data types, validation rules and the dose schedule
internal/config/         Loads and validates configuration
internal/demo/           Demo data for one person
internal/mcpserver/      MCP server, tools, prompts and resources
internal/mcpserver/ui/   MCP App card (HTML)
internal/server/         HTTP handler and server lifecycle
internal/store/          Saves care data to a JSON file
internal/version/        Build metadata
scripts/e2e.sh           End-to-end test
```

## Troubleshooting

| Problem | Solution |
|---|---|
| `"0.0.0.0" is not a loopback host` | Use `127.0.0.1` in `KINHAVEN_ADDR`, or set `KINHAVEN_ALLOW_PUBLIC_BIND=true`. |
| `address already in use` | Another process uses the port. Set a different port in `KINHAVEN_ADDR`. |
| `unknown IANA timezone` | Use a name from the IANA database, for example `Asia/Kolkata`. |
| `go.mod requires go >= 1.25.0` | Install Go 1.25 or later. |
| MCP request fails with `403` and `origin not allowed` | Add the page origin to `KINHAVEN_ALLOWED_ORIGINS`. |
| MCP request fails with `400 Unsupported protocol version` | Use a client that supports MCP protocol version 2025-11-25 or later. |
| Tool returns `more than one person is cared for` | Call `list_people`, then pass `recipient_id`. |
| `Amazon SNS needs an AWS region` at startup | Set `AWS_REGION`, or set `KINHAVEN_ALERTS=log`. |
| Alerts show in the log but no SMS arrives | Check that the phone number is verified in the SNS SMS sandbox and that the account has an SMS spending limit above zero. |
| `Amazon Bedrock needs an AWS region` | Set `AWS_REGION`, or set `KINHAVEN_SIM_MODEL=mock`. |
| Bedrock returns `AccessDeniedException` | Check the IAM policy and that the model is available in your account and Region. |
| The microphone button is disabled | Use Chrome or Edge. Other browsers do not have the Web Speech API. You can type requests instead. |
| Simulator says `connect to ...` | Start the server first, and check `KINHAVEN_SIM_MCP_URL`. |
| `make seed` fails with `the data store already has people in it` | Stop the server and remove `data/kinhaven.json`, or set `KINHAVEN_DATA_DIR` to an empty folder. |
| The demo data is gone after the server starts | The server was in operation when you ran `make seed`. Stop the server, seed the data again, then start the server. |
| SMS to an Indian phone number does not arrive | Register a sender ID, an entity ID and a template ID for India. See the link in Related documentation. |
| `log_dose` returns `already recorded` | Pass `scheduled_at` from `get_schedule` to change the record for one dose. |

## Related documentation

- [MCP specification 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25)
- [MCP Streamable HTTP transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Inspector](https://github.com/modelcontextprotocol/inspector)
- [MCP Apps specification](https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx)
- [Amazon Bedrock Converse API](https://docs.aws.amazon.com/bedrock/latest/userguide/conversation-inference.html)
- [Amazon SNS SMS messaging](https://docs.aws.amazon.com/sns/latest/dg/sns-mobile-phone-number-as-subscriber.html)
- [SMS sender ID registration for India](https://docs.aws.amazon.com/sms-voice/latest/userguide/registrations-sms-senderid-india.html)

## License

Apache License 2.0. See [LICENSE](LICENSE).
