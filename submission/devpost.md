# Kinhaven: Devpost submission text

Copy each section into the matching Devpost field. This folder is not part of
the repository. Do not upload it to GitHub.

## Project name

Kinhaven

## Tagline

An Alexa+ caregiving companion. It tracks medicines and check-ins for an older
parent and tells the family when a dose is missed.

## Track and mini challenges

- Track: Alexa+
- Mini challenges: AWS Builder, Open Source

## About the project

### Inspiration

Many older parents live alone, and their children live in another city or
another country. The children call each day and ask the same questions: "Did
you take your tablets? How do you feel?" The parent often forgets a dose, and
the family finds out days later. A voice assistant in the kitchen can ask these
questions at the correct time, keep a record, and tell the family when
something goes wrong.

### What it does

Kinhaven is a self-hosted MCP server for Alexa+. It keeps a care record for one
or more people and gives Alexa+ the tools to read and change that record.

- The parent asks "What do I take now?" or says "I took my Metformin." Alexa+
  reads the schedule or records the dose.
- The parent says how they feel. Kinhaven records a check-in with a mood and a
  note.
- A family member asks "Has Mom taken her tablets?" or "How was her week?"
  Kinhaven gives a short spoken answer and an MCP App card for the screen.
- When a dose is not taken within the grace period, Kinhaven sends one SMS to
  each family contact through Amazon SNS. It sends only one SMS for each missed
  dose.

The schedule card has a **Mark taken** button. The card calls the `log_dose`
tool directly through the MCP Apps protocol, so the parent can touch the screen
instead of speaking.

The gated Alexa+ tools are not available to hackathon participants. For this
reason, the project includes a web simulator in the style of an Echo Show. The
simulator connects to the MCP server as an MCP client, and an agent on Amazon
Bedrock answers the requests. The simulator uses the Web Speech API for voice
input and spoken replies, and it hosts the MCP App cards in a sandboxed iframe.

### How we built it

- **MCP server (Go).** The server uses the official MCP Go SDK and the
  Streamable HTTP transport at `/mcp`, with MCP specification 2025-11-25. It
  has 10 tools, 2 prompts (`daily_briefing` and `weekly_family_update`), and a
  care-plan resource template.
- **Security.** The server binds to `127.0.0.1` by default. It validates the
  `Origin` header and sends HTTP 403 for a foreign origin. It also rejects an
  unsupported `MCP-Protocol-Version` header with HTTP 400. Tool annotations
  mark which tools are read-only, idempotent or destructive.
- **Schedule engine.** Each dose slot is calculated in the timezone of the
  person, so the schedule stays correct across daylight saving changes. A slot
  is upcoming, due, taken, skipped or missed.
- **MCP App.** One self-contained HTML card shows the daily schedule and the
  weekly summary. Tools link to the card through `_meta.ui.resourceUri`.
- **Agent.** The simulator agent uses the Amazon Bedrock Converse API with tool
  use. The default Bedrock model is Claude Haiku 4.5 through a global
  inference profile. A rule-based mock model lets anyone run the demo without
  an AWS account.
- **Alerts.** A watcher checks for missed doses each minute. It sends the SMS
  through Amazon SNS as a transactional message, and it records each alert
  only after the send is successful.
- **Storage.** A JSON file with `0600` permissions. Each write goes to a
  temporary file first and then replaces the old file in one step.
- **Tests.** Unit tests cover about 90% of the statements. An end-to-end script
  builds the binaries, seeds demo data, starts both servers and runs a full
  conversation.

### Challenges we ran into

- We did not have access to the Alexa+ Web Simulator or the MCP Toolkit. We
  built our own simulator and MCP Apps host to show the experience.
- The Bedrock Converse API returns tool input as a Smithy document. A direct
  decode into a Go map failed. We marshal the document to JSON first, and then
  decode the JSON.
- Spoken answers must be short, but the screen can show more. Each tool result
  has two parts: one sentence for speech and a JSON payload for the card.
- A missed dose must cause one SMS, not one SMS each minute. The watcher keeps
  a record of each alert by contact, medicine and slot.

### Accomplishments that we're proud of

- A standard MCP server that works with any MCP client, for example MCP
  Inspector, and not only with our simulator.
- An MCP App card that records a dose without a round trip through the model.
- A demo that operates with no cloud account, and that changes to Amazon
  Bedrock and Amazon SNS with two environment variables.

### What we learned

- How MCP Apps hosts and views communicate with `ui/initialize`, tool input,
  tool results and size changes.
- How to design tool results for a voice-first device with a screen.
- How to use the Bedrock Converse API for a loop of tool calls.

### What's next for Kinhaven

- Test the server in the real Alexa+ environment when access is available.
- Send proactive reminders when a dose is due.
- Give each family member a weekly summary by email or SMS.

## Built with

Go, Model Context Protocol, MCP Apps, MCP Go SDK, Amazon Bedrock, Amazon SNS,
AWS SDK for Go v2, HTML, JavaScript, Web Speech API

## Testing instructions

The demo needs Go 1.25 or later. It does not need an AWS account.

```sh
git clone https://github.com/rohitshukla001/AmazonDeveloperHackathon.git
cd AmazonDeveloperHackathon
cp .env.example .env
make check
make seed
make run
```

In a second terminal, run `make sim`. Then open `http://127.0.0.1:8090` in
Chrome or Edge.

To use Amazon Bedrock, set `AWS_REGION` and
`KINHAVEN_SIM_MODEL=global.anthropic.claude-haiku-4-5-20251001-v1:0` before
`make sim`. To send real SMS, set `KINHAVEN_ALERTS=sns` before `make run`.

To run the end-to-end test, run `make e2e`.

## Pre-existing work

None. All code in the repository was written during the submission period.

## Product feedback

### Which tools, APIs and SDKs did you use, and for what?

- **MCP specification 2025-11-25 and the MCP Go SDK v1.8.0:** the server, the
  Streamable HTTP transport, tools, prompts and resources.
- **MCP Apps extension:** the schedule and summary card.
- **Amazon Bedrock Converse API (Claude Haiku 4.5):** the agent in the Alexa+
  simulator, with tool use.
- **Amazon SNS:** transactional SMS to family contacts for missed doses.
- **AWS SDK for Go v2:** the Bedrock and SNS clients.
- **MCP Inspector:** manual tests of the server.
- **Claude:** an AI coding assistant for code, tests and documents. The
  hackathon rules permit LLM tools.

### What worked well?

- The Alexa+ track accepts a self-hosted MCP server. We used standard MCP, so
  the same server works with any MCP client.
- The Bedrock Converse API uses one message format for all models. The loop of
  tool calls was easy to write and to test with a fake client.
- The SNS `Publish` call sends an SMS in one request, with no topic setup.
- The AWS SDK for Go v2 loads the Region and credentials from the standard
  places.

### What needs work?

- The Alexa+ tools are gated. Participants cannot test how Alexa+ calls the
  tools, speaks the results or shows MCP Apps on a screen.
- There is no public table that shows which MCP features Alexa+ supports, for
  example prompts, resources and MCP Apps.
- For a new Claude model on Bedrock, the Converse call needs an inference
  profile ID, not the base model ID. The error message does not give the
  correct ID.
- The Go SDK returns Bedrock tool input as a Smithy document. It is not clear
  how to decode it into a Go map.
- SMS to Indian phone numbers needs a sender ID, an entity ID and a template
  ID. A developer in India cannot send a test SMS to their own number quickly.

### How was the onboarding?

The Devpost pages explain the requirements and the judging criteria well. Two
items were not clear. The Dates page and the Official Rules give different
judging start dates. Also, the Open Source mini challenge says "additional," and
it is not clear if a new primary project qualifies. AWS onboarding for Bedrock
and SNS was fast, because the console and the SDK documentation are complete.

### Would you build with these devices and services again?

Yes. Alexa+ with MCP is a good fit for caregiving, because older people find
voice easier than an app. We want to test Kinhaven on a real Echo Show when the
Alexa+ tools become available.

## Feature requests

| Feature | Description | Relevance | Priority |
|---|---|---|---|
| Public Alexa+ Web Simulator | Give hackathon participants access to the Web Simulator, or a limited version of it. | Teams can test real Alexa+ behavior and do not have to build a simulator. | High |
| MCP Apps on Echo Show | Show MCP App cards on Echo Show, and document the host context: theme, screen size and input method. | Caregiving answers need a screen for schedules and summaries. | High |
| Proactive reminders from MCP servers | Let an MCP server ask Alexa+ to speak a reminder at a time that the server sets. | A medicine reminder must start from the server, not from the user. | Medium |
| Alexa+ MCP capability table | Publish which MCP features Alexa+ supports and which protocol versions it accepts. | Developers can design for the correct features from the start. | Medium |
| SNS test path for India | Let a developer send a small number of test SMS to a verified Indian number without DLT registration. | Developers in India can test SMS features before a production launch. | Low |

## Friction log

| Task | Steps | Expected result | Actual result | Severity | Workaround | Suggestion |
|---|---|---|---|---|---|---|
| Test the MCP server with Alexa+ | 1. Read the Alexa+ track resources. 2. Look for the Web Simulator and the MCP Toolkit. | Access to a simulator that calls a self-hosted MCP server. | The tools are gated. Participants do not get access. | High | We built a web simulator that acts as an MCP client, with an agent on Amazon Bedrock. | Give participants a limited simulator, or publish a reference MCP client that acts like Alexa+. |
| Show an MCP App card on Alexa+ | 1. Build an MCP App resource. 2. Link tools to it with `_meta.ui.resourceUri`. 3. Look for Alexa+ documentation about MCP Apps. | Documentation that tells if Alexa+ shows MCP Apps on Echo Show. | We did not find this information. | Medium | We wrote our own MCP Apps host in the simulator. | Publish a table of the MCP features that Alexa+ supports. |
| Call Claude Haiku 4.5 through the Bedrock Converse API | 1. Copy the model ID from the model card. 2. Call `Converse` with that ID. | The call returns a response. | The call needs an inference profile ID, for example `global.anthropic.claude-haiku-4-5-20251001-v1:0`. | Medium | Use the inference profile ID. | Show the inference profile ID on the model card, and put the correct ID in the error message. |
| Decode tool input from a Converse response in Go | 1. Get a `ToolUseBlock` from the response. 2. Call `UnmarshalSmithyDocument` into a `map[string]any`. | A Go map with the tool arguments. | The decode failed with an "unsupported json type" error. | Medium | Call `MarshalSmithyDocument`, then decode the JSON with `encoding/json`. | Add a Go example that decodes tool input to the Converse tool-use documentation. |
| Send a missed-dose SMS to an Indian phone number | 1. Set up Amazon SNS in `ap-south-1`. 2. Call `Publish` with an Indian phone number. | The phone gets the SMS. | India needs a registered sender ID, entity ID and template ID before SMS delivery. A new account is also in the SMS sandbox. | Medium | Use the log sender for the demo, or use a verified number in the SMS sandbox. | Give a small test path for verified Indian numbers, and link the India rules from the SNS SMS quick start. |
| Find the judging start date | 1. Read the Dates page. 2. Read the Official Rules. | One judging start date. | The Dates page shows 26 October. The Official Rules show 9 November. | Low | Keep the repository and the demo available until 20 November. | Use the same dates on all pages. |
