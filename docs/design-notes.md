# Design Notes for Spec v1 Implementation

## Option A: Monolithic Loop
- Single `main` package with inlined logic for config loading, Telegram polling, command execution, sessions, and heartbeats.
- Minimal types beyond structs mirroring config; session map protected by mutex.
- Pros: fastest to hack together, fewer packages.
- Cons: tightly coupled, hard to test individual behaviors (heartbeat, Codex execution, session expiry). Refactoring needed for future transports.

## Option B: Modular Components with Interfaces (Chosen)
- Split into packages: `config`, `telegram`, `codex`, `session`, and `service` coordinating them.
- Define small interfaces (`Provider`, `AIClient`, `SessionStore`, `Clock`) so polling loop and heartbeat scheduler can be unit tested with fakes.
- CLI commands (`start`, `heartbeat`, `status`) live under `cmd/cma`, wired through a `Service` struct that depends on the interfaces.
- Pros: aligns with "deep modules" guidance—simple APIs for callers (`Service.Start()` etc.) while hiding polling and command execution complexity. Easier unit testing by swapping in fakes.
- Cons: More files and wiring compared to monolithic approach; slight upfront design cost.

We proceed with Option B to keep complexity contained behind clear interfaces and enable meaningful tests.
