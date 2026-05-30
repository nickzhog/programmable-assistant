# Telegram OpenCode Bot

A Telegram bot that acts as a wrapper for the `opencode` CLI. This bot allows users to interactively run AI planning and building sessions against local codebases directly from Telegram.

## Features

- **Interactive Sessions:** Create and manage sessions tied to specific directories in your local codebase.
- **AI Planning & Building:** Run `opencode` in plan and build modes via Telegram commands.
- **Real-time Output:** Streams standard output and standard error from the `opencode` CLI directly into Telegram message edits.
- **File Management:** Browse your local codebase, change directories, download files, and upload documents/photos straight to your working directory.
- **Provider Configuration:** Dynamically switch AI providers, models, and thinking presets using inline keyboards.
- **Secure Access:** Whitelist specific Telegram User IDs to restrict access to the bot.

## Architecture

- **Handlers:** Built with `telebot.v3` for robust Telegram command and callback handling.
- **Runner:** Spawns the `opencode` CLI as a subprocess and streams output securely. Supports graceful process group termination.
- **Storage:** Uses a single `bbolt` database file for persistence (sessions, navigation paths, finite state machine states, etc.).
- **FSM:** Implements a finite state machine for multi-step interactive flows (e.g., selecting AI providers).

## Prerequisites

- **Go 1.24** or higher
- The `opencode` CLI installed and available in your system's PATH.
- A Telegram Bot Token (obtainable from [@BotFather](https://t.me/botfather)).

## Configuration

The bot uses a `config.toml` file for configuration. Here is an overview of the required settings:

- `allowed_users`: A required, non-empty list of whitelisted Telegram User IDs.
- `default_opencode_dir`: The default working directory for new sessions.
- `[opencode.aliases.<name>]`: Presets for providers, models, and thinking configurations used by the `/plan` and `/build` commands.
- `[opencode.defaults]`: The default plan and build aliases to use when a new session is created.

You can copy the `.env.example` file or create your configuration appropriately.

## Installation & Running

There are no build scripts or Makefiles required. Simply run the bot using Go:

```bash
# Run with default settings (expects config.toml and bot.db in the current directory)
export TELEGRAM_BOT_TOKEN="your-telegram-bot-token"
go run cmd/bot/main.go

# Run with custom config and database paths
go run cmd/bot/main.go -config /path/to/config.toml -db /path/to/bot.db
```

## Usage

Once the bot is running and you have started a chat with it, you can use the following commands:

- `/new_session <name>` - Creates a new session at the current navigator path and assigns default plan/build aliases.
- `/plan <prompt>` - Runs `opencode` in plan mode using the current session's plan alias. Offers a "Create Build session from plan" inline button upon completion.
- `/build <prompt>` - Runs `opencode` in build mode using the current session's build alias.
- `/provider` - Opens an interactive inline keyboard to set or change the plan/build alias for the current session.
- `/files` - Opens a paginated file browser (12 entries per page). Allows downloading files directly to Telegram.
- `/cd <path>` - Changes the current navigator directory.
- **Uploads** - Sending a document or photo to the bot will automatically save it to the current navigator directory.
