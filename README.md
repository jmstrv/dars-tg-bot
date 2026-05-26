# Telegram Lesson Slot Booking Bot (dars-tg-bot)

Simple Telegram bot to manage lesson/day/time slot booking inside a group chat.

Requirements
- Go 1.18+
- A Telegram bot token from BotFather

Quick start

1. Set up the project dependencies:

```bash
go mod tidy
```

2. Export your bot token:

```bash
export BOT_TOKEN="<your BotFather token>"
```

3. Run the bot:

```bash
go run main.go
# or build and run
go build -o dars-bot && ./dars-bot
```

Usage

- Add the bot to your group and make it an admin (required for reading members, pinned messages, etc.).
- The bot auto-discovers the chat and admins.
- On first interaction the bot shows a language picker (EN / UZ / RU). Users can change language anytime with the `/language` command.

Admin commands
- `/setup` — runs the admin setup wizard:
	- Step 0 — language picker (only if language not chosen)
	- Step 1 — select lesson days (inline toggle buttons)
	- Step 2 — set time slots (free-text, e.g. `08:00 09:00 10:00`)
	- Step 3 — confirm & apply

Environment
- `BOT_TOKEN` — Telegram bot token (required)
- `WEBHOOK_PATH` — optional webhook path, default `/telegram-webhook`
- `WEBHOOK_URL` — optional full webhook URL; if not set, Render's `RENDER_EXTERNAL_URL` is used

Render deployment
- Root Directory: `.`
- Build Command: `go build -o dars-bot .`
- Start Command: `./dars-bot`
- Webhook URL: `https://<your-app>.onrender.com/telegram-webhook`

Notes
- No additional configuration files required for basic use.
- If you change the module name, update `go.mod` accordingly.

Questions or contributions
- Open an issue or a pull request in the project repository.

