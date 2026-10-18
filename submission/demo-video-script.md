# Kinhaven: demo video script

Target length: 2 minutes 45 seconds. The video must be shorter than 3 minutes.
Do not add music.

## Preparation

1. Set `KINHAVEN_TIMEZONE=Asia/Kolkata` in `.env`.
2. Record after 10:00 local time, so that the schedule shows taken, missed and
   due doses.
3. Stop the server, remove `data/kinhaven.json`, and run `make seed`.
4. In terminal 1, run `make run`.
5. In terminal 2, set `AWS_REGION` and `KINHAVEN_SIM_MODEL` for Amazon Bedrock,
   and run `make sim`.
6. Open `http://127.0.0.1:8090` in Chrome. Allow the microphone.
7. Open MCP Inspector in a second tab, and connect it to
   `http://127.0.0.1:8080/mcp`.
8. Make the browser window 1280 × 800. Close other tabs and notifications.

## Script

| Time | Screen | Narration |
|---|---|---|
| 0:00 – 0:15 | The simulator home screen with the clock and the suggestions. | "My mother lives alone. Each day I call and ask the same thing: did you take your tablets? Kinhaven lets Alexa+ ask, record and tell the family." |
| 0:15 – 0:40 | Press the microphone. Say "Has Mom taken her tablets today?" The schedule card opens. | "Kinhaven is an MCP server for Alexa+. Alexa+ calls the `get_schedule` tool. It speaks one short sentence, and the screen shows an MCP App card with each dose." |
| 0:40 – 0:55 | Point to the missed dose and the "family alerted" label. Switch to terminal 1 and show the missed-dose alert line. | "This dose is missed. The watcher sent one SMS to Priya through Amazon SNS. It sends only one SMS for each missed dose." |
| 0:55 – 1:15 | Select **Mark taken** on a due dose. Open the **MCP activity** panel. | "The card calls the `log_dose` tool directly through the MCP Apps protocol. The model is not in this step, so the update is immediate." |
| 1:15 – 1:35 | Say "I took my Metformin." Then say "I feel a bit tired today." | "The parent can also speak. The agent finds the correct dose slot and records it. It records the check-in with a mood and a note." |
| 1:35 – 1:55 | Say "How was her week?" The summary card opens with bars, moods and notes. | "For the family, `get_adherence_summary` gives the week: doses taken, missed and skipped, for each medicine, with the check-in notes." |
| 1:55 – 2:15 | Show the MCP activity panel. Then show terminal 2 with the Bedrock model ID. | "The agent in this simulator uses the Amazon Bedrock Converse API with tool use. The Alexa+ tools are gated, so this simulator acts as the Alexa+ client." |
| 2:15 – 2:35 | Switch to MCP Inspector. Show the tools list, the prompts and the care-plan resource. | "This is a standard MCP server on Streamable HTTP, with specification 2025-11-25. It validates the Origin header, and it binds to localhost by default." |
| 2:35 – 2:45 | Show the GitHub repository page with the license. | "Kinhaven is open source under Apache 2.0. Clone it, run `make seed` and `make run`, and try it with no AWS account." |

## Checks before you upload

1. The video is shorter than 3 minutes.
2. The video shows the simulator, the MCP App card and the Bedrock model ID.
3. No full phone number shows on the screen.
4. No AWS access key or account ID shows on the screen.
5. The video has no music and no third-party logos.
6. Upload the video to YouTube or Vimeo as public.
