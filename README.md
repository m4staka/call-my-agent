# Call My Agent (cma)

A small Go CLI that polls a Telegram bot, forwards allowed chats to the Codex CLI, and replies with the model output while maintaining per-chat sessions. The CLI also supports manual heartbeats and a simple status report.

## Requirements
- Go 1.22+
- Telegram bot token supplied via the `TELEGRAM_BOT_TOKEN` environment variable (not required for `status`)
- OpenAI API key supplied via the `OPENAI_API_KEY` environment variable for voice transcription
- `codex` CLI installed and logged in (required when using command mode replies)
  - Install: follow the [codex documentation](https://github.com/codex-ai/codex) for installation instructions
  - Login: run `codex login` to authenticate before running `cma` commands

## Configuration
The CLI reads a JSON config file (default `config.json`):

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
      "agent": "codex",
      "cwd": "/home/me/projects/demo",
      "timeoutSeconds": 600,
      "session": {
        "scope": "per-chat",
        "idleMinutes": 60,
        "resetTriggers": ["/new"],
        "heartbeatIdleMinutes": 240,
        "maxMessages": 40
      }
    }
  },
  "logging": {
    "level": "info",
    "file": "/tmp/cma.log"
  },
  "sessionStorePath": "/home/me/.cma/sessions.json"
}
```

Key notes:
- **Allowed chats**: only chat IDs listed in `inbound.allowFrom` are processed.
- **Reply modes**: `static` returns `staticText`; `command` shells out via the configured agent tool (`inbound.reply.agent`).
  - `codex` (default) runs `codex exec "<task>"` using the task built from the message and session context.
  - `pi` runs the [Pi coding agent CLI](https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent) with `pi -p --no-session ...`.
- **Sessions**: messages are tracked per chat with idle expiry, `/new` resets history, and `maxMessages` bounds the stored context before persisting it to disk.
- **Working directory**: `inbound.reply.cwd` (optional) runs all Codex commands, including heartbeats, from a specific directory; omit it to use `cma`'s current directory.
- **Session store**: conversations are serialized to `sessionStorePath` after each change (defaults to `~/.cma/sessions.json`), so `start`, `heartbeat`, and `status` share the same context.
- **Heartbeats**: set `inbound.heartbeatMinutes` to `0` to disable; heartbeat prompts follow the `HEARTBEAT TELEGRAM` convention and suppress `HEARTBEAT_OK`.
- **Logging**: `logging.level` supports `silent|error|warn|info|debug`; `logging.file` controls the log destination (default `/tmp/cma.log`).

## Usage
Build the binary and run a command (default config path is `config.json`):

```bash
make build
TELEGRAM_BOT_TOKEN=xxx ./cma start -config path/to/config.json
TELEGRAM_BOT_TOKEN=xxx ./cma heartbeat -config path/to/config.json
./cma status -config path/to/config.json
./cma status -config path/to/config.json --json --limit 5
```

`start` runs the polling loop, `heartbeat` runs a single heartbeat pass, and `status` prints session summaries.

### Voice messages
- Telegram voice and audio messages are downloaded via `getFile` and sent to the OpenAI Whisper API for transcription.
- The transcribed text is processed like any other inbound message; on failure, the bot replies with an apology.

## Development
- Format: `make fmt`
- Tests: `make test`

See `docs/design-notes.md` for the chosen modular design and `docs/spec-v1.md` for the full product specification.
