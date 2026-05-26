// Telegram Lesson Slot Booking Bot
//
// Setup:
//   1. go mod init lesson-bot
//   2. go get github.com/go-telegram-bot-api/telegram-bot-api/v5
//   3. export BOT_TOKEN=<your BotFather token>
//   4. go run main.go
//
// No other configuration needed. Add the bot to your group and make it admin.
// The bot discovers the chat and admins automatically.
//
// First interaction with any user triggers a language picker (EN / UZ / RU).
// Every subsequent message is delivered in that user's chosen language.
// Users can change their language at any time with /language.
//
// Admin setup wizard (/setup):
//   Step 0 — Language picker  (shown only if language not yet chosen)
//   Step 1 — Lesson days      (inline toggle buttons)
//   Step 2 — Time slots       (free-text "08:00 09:00 10:00")
//   Step 3 — Confirm & apply# dars-tg-bot
