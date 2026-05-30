# AGENTS.md

## Project

Telegram bot that wraps the `opencode` CLI. Users interact via Telegram commands to run AI planning/building sessions against local codebases.

## Commands

```bash
make run                          # loads .env automatically
go run ./cmd/bot/main.go          # requires TELEGRAM_BOT_TOKEN in shell env
```

Requires `TELEGRAM_BOT_TOKEN` env var.

## Config (`config.toml`)

- `allowed_users` — Telegram user IDs whitelist (required, non-empty)
- `default_opencode_dir` — default working directory for sessions
- `[opencode.aliases.<name>]` — provider/model/thinking presets used by `/plan` and `/build`
- Default aliases plan/build are set via `[opencode.defaults]`

## Architecture

```
cmd/bot/main.go          → entrypoint: loads config, opens bbolt, creates bot+runner+fsm+handler
internal/handler/        → Telegram command/callback handlers (telebot.v3)
internal/runner/         → spawns `opencode run --model <provider>/<model> --prompt <prompt>` as subprocess, streams stdout/stderr back
internal/store/bbolt.go  → bbolt persistence (sessions, nav paths, FSM states, active session, callback refs)
internal/fsm/            → finite state machine for multi-step flows (e.g. /provider alias selection)
internal/session/        → session model (id, name, workdir, plan/build aliases, status)
internal/navigator/      → paginated directory listing for /files
internal/sanitizer/      → ANSI escape code stripping from opencode output
internal/config/         → TOML config loading + validation
```

## Key flows

- `/new_session <name>` — creates session at current navigator path, sets plan/build aliases from config defaults
- `/plan <prompt>` — runs opencode in plan mode using session's plan alias, streams output, offers "Create Build session from plan" button
- `/build <prompt>` — runs opencode in build mode using session's build alias
- `/provider` — FSM-driven inline keyboard flow to set plan or build alias per session
- `/files` + `/cd <path>` — file browser with pagination (12 entries/page), file download via Telegram document
- Upload (document/photo) — saves to current navigator directory

## Runner

- Spawns `opencode run --format json` as subprocess (JSON mode required for session ID extraction)
- Uses `Setpgid` and kills entire process group on abort
- One concurrent run per session ID
- Output streamed via Telegram message edits (max 4096 chars)
- stdout parsed as JSON events (`{type, sessionID, part:{type,text}}`); stderr stripped of ANSI codes

## Storage

- Single bbolt file (`bot.db` by default)
- Buckets: sessions, nav_paths, fsm_states, active_sessions, callback_refs
- Callback refs use short UUIDs to work around Telegram callback data length limits

## Conventions

- No tests, no CI, no linter configured
- Go 1.24, stdlib `log/slog` for logging
- All handlers in `internal/handler/`, one file per command group
- Callback data format: `namespace:action:payload` (e.g. `fsm:provider:mode:plan`, `abort:<session_id>`)
