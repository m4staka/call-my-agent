# Call My Agent (cma)

A small Go CLI that polls a Telegram bot, forwards allowed chats to the Codex CLI, and replies with the model output while maintaining per-chat sessions. The CLI also supports manual heartbeats and a simple status report.

## Requirements
- Go 1.25+
- Telegram bot token supplied via the `TELEGRAM_BOT_TOKEN` environment variable
- Access to the `codex` CLI when using command mode replies

## Configuration
The CLI reads a JSON config file (default `config.json`). An example aligned with `docs/spec-v1.md`:

```json
{
  "telegram": {
    "pollIntervalSeconds": 2
  },
  "inbound": {
    "allowFrom": ["123456789"],
    "heartbeatMinutes": 15,
    "reply": {
      "mode": "command",
      "staticText": "Thanks for your message!",
      "bodyPrefix": "You are a helpful assistant.",
      "command": ["codex", "exec", "{{.Task}}"],
      "timeoutSeconds": 600,
      "session": {
        "scope": "per-chat",
        "idleMinutes": 60,
        "resetTriggers": ["/new"],
        "heartbeatIdleMinutes": 240
      }
    }
  },
  "logging": {
    "level": "info",
    "file": "/tmp/cma.log"
  }
}
```

Key notes:
- **Allowed chats**: only chat IDs listed in `inbound.allowFrom` are processed.
- **Reply modes**: `static` returns `staticText`; `command` shells out to the command template using `{{.Task}}`, `{{.ChatID}}`, `{{.Body}}`, and `{{.BodyStripped}}` placeholders.
- **Sessions**: messages are tracked per chat with idle expiry; `/new` clears history.
- **Heartbeats**: set `inbound.heartbeatMinutes` to `0` to disable.

## Usage
Build the binary and run a command (default config path is `config.json`):

```bash
make build
TELEGRAM_BOT_TOKEN=xxx ./cma start -config path/to/config.json
TELEGRAM_BOT_TOKEN=xxx ./cma heartbeat -config path/to/config.json
./cma status -config path/to/config.json
```

`start` runs the polling loop, `heartbeat` runs a single heartbeat pass, and `status` prints session summaries.

## Development
- Format: `make fmt`
- Tests: `make test`

See `docs/design-notes.md` for the chosen modular design and `docs/spec-v1.md` for the full product specification.
