# Call-my-agent – v1 Spec

## 1. Overview

Call-my-agent is a small Go CLI that:

- Polls a Telegram bot for new messages.
- For each allowed chat, calls **OpenAI Codex CLI** (`codex exec`) as an external process.
- Sends Codex’s response back to Telegram.
- Maintains **per-chat sessions** (context + idle expiry).
- Supports **heartbeats**: periodic background pings to Codex that can trigger proactive messages.

Architecture and configuration are heavily inspired by **warelay** (`steipete/warelay`) but simplified for a single transport (Telegram) and a single agent (Codex CLI). See warelay’s README sections:

- “Main Features”, “Command Cheat Sheet” and “Auto-reply config (`~/.warelay/warelay.json` )” for structure and concepts. 0  

---

## 2. Goals and Non-Goals (v1)

### 2.1 Goals

- Telegram-only integration via **polling** using the Telegram Bot API (`getUpdates` / `sendMessage`). 1  
- 1:1 **direct chats** (individual chat IDs), no groups/supergroups for v1.
- Simple, configurable **auto-reply engine**:
  - `mode: "static"` (template reply) or `mode: "command"` (Codex CLI).
- Agent selection via `inbound.reply.agent` (e.g., `codex`, `pi`) to choose which tool handles replies.
- **Codex CLI integration** in non-interactive mode using `codex exec "<task>"`, reading final result from `stdout`. 2  
- Per-chat **sessions**:
  - `scope: "per-chat"`, `idleMinutes`, `/new` resets (similar to warelay’s sessions). 3  
- **Heartbeat** loop:
  - Global `heartbeatMinutes` scheduler.
  - Heartbeat prompt convention with response suppression when Codex returns `HEARTBEAT_OK`, modeled on warelay’s heartbeat behavior. 4
- Basic **access control** (`allowFrom` chat IDs).
- Basic **logging** and a `status` command similar in spirit to `warelay status`. 5
- **Voice transcription** for Telegram `voice` and `audio` messages using the OpenAI Whisper API (audio fetched via `getFile`).

### 2.2 Non-Goals (explicitly out of scope for v1)

- **No WhatsApp/Twilio** support.
- **No webhooks** (Telegram `setWebhook`) – polling only.
- **No rich media beyond audio**:
  - Voice/audio are supported; photos, documents, and stickers remain out of scope.
- **No group chat logic** (no threads in groups, no multi-user routing).
- **No Codex SDK, Agents SDK, or MCP**:
  - v1 only shells out to `codex` CLI; no TypeScript SDK or MCP server usage. 7  
- **No advanced delivery tracking / typing indicators** (warelay’s Twilio-specific features are ignored). 8  
- **No multi-provider abstraction** beyond what’s needed for Telegram; the design should be pluggable but only Telegram is implemented.
- **No UI or dashboard** – CLI + logs only.

---

## 3. High-Level Architecture

### 3.1 Components

1. **CLI Layer**
   - Binary name: `cma` (placeholder).
   - Subcommands (inspired by `warelay`’s “Command Cheat Sheet”): 9  
     - `cma start` – main long-running worker (polls Telegram, processes auto-replies).
     - `cma heartbeat` – runs one heartbeat pass across sessions (optional, can be triggered manually).
     - `cma status` – prints recent sessions/messages from the store.

2. **Config Loader**
   - Reads a single config file at startup, analogous to `~/.warelay/warelay.json`, and combines it with required environment variables. 10  
   - Telegram bot token is **not** stored in the config file; it is read from an environment variable (e.g. `TELEGRAM_BOT_TOKEN`) for better secret handling.
   - Proposed structure (conceptual):

     ```text
    telegram:
      pollIntervalSeconds: 2          # bot token from env (e.g. TELEGRAM_BOT_TOKEN)

     inbound:
       allowFrom: ["123456789"]         # Telegram chat IDs as strings
       reply:
         mode: "command" | "static"
         staticText: "..."              # if mode == static
         bodyPrefix: "..."              # system prompt prefix
        agent: "codex"                 # Agentic tool: codex (default) or pi
         cwd: "/home/user/project"      # optional working dir for codex CLI
         timeoutSeconds: 600

         session:
           scope: "per-chat"
           idleMinutes: 60
           resetTriggers: ["/new"]
           heartbeatIdleMinutes: 240     # optional override

         heartbeatMinutes: 0 | N        # 0 = disabled

     logging:
       level: "info"
       file: "/tmp/cma.log"
     ```

   - Keep it conceptually close to warelay’s `inbound.reply`, `session`, `heartbeatMinutes`, `logging` blocks so the implementer can look at the README and copy patterns. 11  

3. **Telegram Provider (Transport)**

   Responsibilities:

   - Poll Telegram using `getUpdates` with `offset` and `timeout` parameters. 12  
   - Map each `Update` to an internal `InboundMessage` with fields:
     - `ChatId` (int64 or string),
     - `MessageId`,
     - `Text`,
     - `Timestamp`.
   - Send replies via `sendMessage(chat_id, text)`.

   Internally, this mirrors warelay’s provider abstraction (Twilio/Web) but reduced to Telegram-only. 13  

4. **Codex Client (AI integration)**

   Responsibilities:

   - Build the `task` string for `codex exec` based on:
     - `bodyPrefix`,
     - session context (optional N last messages),
     - current user message (with `/new` stripped if needed).
  - Run the selected CLI (`codex exec "<task>"` for Codex; `pi -p --no-session` for Pi) as a subprocess (agent comes from config).
     - Use the docs’ guarantee that progress goes to stderr and the **final agent message** goes to stdout. 14  
   - Respect `timeoutSeconds`.
   - Strip trailing whitespace; return the final string to the caller.

   For architecture, keep this very similar to warelay’s “Claude CLI setup (how we run it)” section, but replace `claude` with `codex exec`. 15  

5. **Session Store**

   Responsibilities:

   - Maintain sessions keyed by `ChatId`.
   - Track:
     - `SessionId` (UUID),
     - `ChatId`,
     - `CreatedAt`, `UpdatedAt`,
     - `Messages` (bounded list of recent `(role, content)` pairs),
     - Optional `Metadata` (for future Codex thread IDs – not required in v1).

   - Enforce:
     - `idleMinutes`: if now − `UpdatedAt` > `idleMinutes`, new session.
     - `resetTriggers`: if user text starts with `/new` (or configured trigger), new session (similar to warelay’s `/new` behavior). 16  
   - Storage: simple local file (e.g., JSON) refreshed on each change, similar to warelay’s `~/.warelay/sessions.json` concept. 17  

6. **Heartbeat Scheduler**

   Responsibilities:

   - If `heartbeatMinutes > 0`, start a `time.Ticker`.
   - On each tick:
     - Iterate over active sessions.
     - Skip sessions older than `session.heartbeatIdleMinutes` (or `idleMinutes` if not set), mirroring warelay’s “heartbeat skips do not bump updatedAt” rule. 18  
     - For each eligible session, build a heartbeat task:

       > “HEARTBEAT TELEGRAM\n\n[short summary / last messages]\n\nIf there is nothing useful to tell the user, reply with exactly HEARTBEAT_OK.”

     - Call Codex CLI via `codex exec`.
     - If result is `HEARTBEAT_OK` → log only, no Telegram send.
     - Else → send message to respective chat via Telegram Provider.

   - Also support a one-off `cma heartbeat` CLI command that runs the same logic once (similar to `warelay heartbeat`). 19  

7. **Access Control & Filtering**

   - `allowFrom` is checked before creating/using a session:
     - If list is non-empty → only process messages where `ChatId` is in that list.
     - If `allowFrom` is `"*"` → accept all chats.
   - This is directly analogous to warelay’s inbound access control. 20  

8. **Logging & Status**

   - File logging to a configurable path (default `/tmp/cma.log`), with levels similar to `silent | error | warn | info | debug`. 21  
   - `cma status`:
     - Reads the session store and prints:
       - recent sessions (last N by `UpdatedAt`),
       - last message and last Codex reply per session.
     - Optionally a `--json` flag, mirroring `warelay status --json`. 22  

---

## 4. Data & Interface Sketches (informal)

*(High-level, not strict Go code; purpose is to clarify responsibilities.)*

- **InboundMessage**
  - `ChatId` (string/int)
  - `MessageId`
  - `Text`
  - `Timestamp`

- **Session**
  - `Id` (UUID string)
  - `ChatId`
  - `CreatedAt`, `UpdatedAt`
  - `Messages: [ { Role: "user" | "assistant", Content: string } ]`

- **Provider (Telegram-only implementation in v1)**
  - `Receive() <-chan InboundMessage` – long-running polling loop.
  - `Send(chatId, text string) error`

- **AIClient (Codex)**
  - `Run(task string, timeout time.Duration) (string, error)`

- **SessionManager**
  - `GetOrCreate(chatId, userText) (*Session, bool wasReset)`
  - `AppendMessage(session, role, content)`
  - `ExpireIdleSessions()`

---

## 5. Behavior Summary (Happy Path)

1. `cma start` starts.
2. Config and session store are loaded.
3. Telegram Provider starts polling and pushes `InboundMessage` objects into the core loop.
4. For each inbound message:
   - Check `allowFrom`.
   - Resolve session via `SessionManager` (consider `idleMinutes` and `/new`).
   - Build Codex task string (prefix + recent context + new user text).
   - Call `codex exec "<task>"` with timeout. 23  
   - Append both user message and Codex reply to session history.
   - Send Codex reply back to Telegram.
5. In parallel, Heartbeat Scheduler runs if enabled:
   - For each fresh-enough session, run a heartbeat task.
   - If Codex returns anything except `HEARTBEAT_OK`, send to Telegram; otherwise just log.

---

## 6. References for Implementer

- **warelay concepts & config:**
  - README – “Main Features”, “Command Cheat Sheet”, “Auto-reply config (`~/.warelay/warelay.json`)”, “Heartbeat pings (command mode)”, and “Logging”. 24  
  - `docs/arthur.md` – example of a proactive personal assistant setup built on warelay + Claude Code (design inspiration for proactive Codex agent). 25  

- **Codex CLI usage:**
  - Codex CLI overview and features: docs “Codex CLI” (install, interactive usage, scripting). 26  
  - Codex SDK docs – “Using Codex CLI programmatically” (explains `codex exec` behavior: progress on stderr, final message on stdout). 27  

- **Telegram Bot API basics:**
  - Telegram Bot API docs for `getUpdates` / `sendMessage` to implement polling + replies. 28  

This spec should be enough for a single developer to implement a focused v1 without accidentally building extra Twilio/WhatsApp features or Codex SDK integrations.
```29

0. https://github.com/steipete/warelay 

1. https://core.telegram.org/bots/api 

2. https://developers.openai.com/codex/cli/reference/ 

3. https://github.com/steipete/warelay/releases 

4. https://stackoverflow.com/questions/3408780/telegram-bot-and-the-method-getupdates 

5. https://core.telegram.org/method/messages.sendMessage 

6. https://developers.openai.com/codex/cli/ 

7. https://github.com/steipete/warelay/actions 

8. https://telegram-bot-sdk.readme.io/reference/getupdates 

9. https://telegram-bot-sdk.readme.io/reference/sendmessage 

10. https://github.com/peterdemin/openai-cli 

11. https://github.com/steipete/warelay/blob/main/AGENTS.md 

12. https://github.com/Cale-Torino/Telegram_Bot_API_Quick_Example 

13. https://stackoverflow.com/questions/31197659/how-to-send-request-to-telegram-bot-api 

14. https://platform.openai.com/docs/api-reference/introduction 

15. https://github.com/steipete/warelay/activity 

16. https://www.postman.com/davtur19/telegram/request/mt6unli/getupdates 

17. https://medium.com/internet-of-technology/how-to-operate-with-openai-command-line-client-b8174746f730 

18. https://github.com/steipete/warelay/blob/main/LICENSE 

19. https://community.latenode.com/t/switching-from-webhook-to-getupdates-in-telegram-bot-api/9806 

20. https://community.openai.com/t/unable-to-run-the-command-line-interface-cli-for-openai/6197 

21. https://github.com/steipete/warelay/blob/main/docs/clawd.md 

22. https://hackage.haskell.org/package/telegram-bot-simple-0.8/docs/Telegram-Bot-API-GettingUpdates.html 

23. https://crates.io/crates/openai-cli 

24. https://github.com/steipete/warelay/blob/main/.npmrc 

25. https://www.youtube.com/watch?v=VqCwI_aEv4o 

26. https://gist.github.com/dideler/85de4d64f66c1966788c1b2304b9caf1 

27. https://www.reddit.com/r/GPT3/comments/s0giqc/openai_commandline_interface/ 

28. https://github.com/steipete/warelay/blob/main/tsconfig.json 
