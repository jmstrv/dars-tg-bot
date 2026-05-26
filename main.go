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
//   Step 3 — Confirm & apply

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ─── i18n ─────────────────────────────────────────────────────────────────────

type Lang string

const (
	LangEN Lang = "en"
	LangUZ Lang = "uz"
	LangRU Lang = "ru"
)

// Strings holds every user-visible string for one language.
type Strings struct {
	// Language picker
	PickLanguage string

	// Generic
	AdminOnly      string
	SetupCancelled string
	NoActiveSetup  string

	// /help
	HelpText string

	// /status
	StatusTitle      string
	StatusGroup      string
	StatusDays       string
	StatusSlots      string
	StatusBookings   string
	StatusNoBookings string
	StatusNoGroup    string

	// Wizard step 1 — days
	WizStep1Title    string
	WizStep1Body     string
	WizDaysNextEmpty string
	WizDaysNext      string

	// Wizard step 2 — hours
	WizStep2Title string
	WizStep2Body  string

	// Wizard step 3 — confirm
	WizStep3Title string
	WizStep3Body  string
	WizApply      string
	WizStartOver  string
	WizApplied    string
	WizRestarted  string

	// Validation errors
	ErrBadTimeFormat string
	ErrInvalidTime   string
	ErrNoSlots       string
	ErrTryAgain      string

	// Broadcast
	BroadcastMsg string

	// Booking feedback (toast alerts — plain text, no Markdown)
	BookedAlert  string // "%s" = slot
	TakenAlert   string // "%s" = slot
	TooSlowAlert string // "%s" = slot

	// Group join
	JoinMsg string

	// Day names (localised)
	DayNames map[string]string

	// /language command
	LanguageChanged string
}

var translations = map[Lang]*Strings{
	LangEN: {
		PickLanguage: "🌐 Please choose your language:",

		AdminOnly:      "🚫 Only group admins can use this command.",
		SetupCancelled: "❌ Setup cancelled.",
		NoActiveSetup:  "ℹ️ No active setup to cancel.",

		HelpText: "📚 *Lesson Slot Bot*\n\n" +
			"Add me to your group and make me admin — I'll detect the chat and admins automatically\\.\n\n" +
			"*Everyone:*\n" +
			"/language — Change your language\n" +
			"/status — Show current config and bookings\n" +
			"/help — Show this message\n\n" +
			"*Group admins only:*\n" +
			"/setup — Configure lesson days & time slots\n" +
			"/broadcast — Send booking keyboard now \\(test\\)\n" +
			"/cancel — Abort in\\-progress setup\n\n" +
			"_Booking keyboards are sent at 18:00 the evening before each lesson day\\._",

		StatusTitle:      "📊 *Bot Status*",
		StatusGroup:      "*Group chat ID:* %s",
		StatusDays:       "*Lesson days:* %s",
		StatusSlots:      "*Time slots:* %s",
		StatusBookings:   "*Current bookings:*\n%s",
		StatusNoBookings: "_None yet_",
		StatusNoGroup:    "_Not in a group yet_",

		WizStep1Title:    "⚙️ *Lesson Setup — Step 1 of 3*",
		WizStep1Body:     "Tap the days when lessons take place\\.\nTap again to deselect\\. Press *Next* when done\\.",
		WizDaysNextEmpty: "⚠️ Select at least one day first",
		WizDaysNext:      "Next →",

		WizStep2Title: "⚙️ *Lesson Setup — Step 2 of 3*",
		WizStep2Body: "Type the available time slots, space\\-separated:\n\n" +
			"`08:00 09:00 10:00 11:00`\n\n" +
			"_Send /cancel to abort at any time\\._",

		WizStep3Title: "⚙️ *Lesson Setup — Step 3 of 3*",
		WizStep3Body: "Please confirm your settings:\n\n" +
			"📅 *Lesson days:* %s\n" +
			"🕒 *Time slots:* %s\n\n" +
			"Tap *Apply* to save, or *Start over* to redo\\.",
		WizApply:     "✅ Apply",
		WizStartOver: "🔄 Start over",
		WizApplied: "✅ *Setup complete\\!*\n\n" +
			"📅 Lesson days: *%s*\n" +
			"🕒 Time slots: *%s*\n\n" +
			"_Booking keyboards will be sent at 18:00 before each lesson day\\._",
		WizRestarted: "🔄 Setup restarted\\. Use /setup to begin again\\.",

		ErrBadTimeFormat: "❌ Bad format `%s` — expected HH:MM",
		ErrInvalidTime:   "❌ Invalid time `%s`",
		ErrNoSlots:       "❌ No valid time slots provided",
		ErrTryAgain:      "Try again, e\\.g\\. `08:00 09:00 10:00`",

		BroadcastMsg: "📅 *Lesson tomorrow\\!* Pick your time slot:",

		BookedAlert:  "✅ Booked %s!",
		TakenAlert:   "❌ %s is already taken!",
		TooSlowAlert: "⚡ Too slow! %s was just taken.",

		JoinMsg: "👋 Hello *%s*\\! I'm your lesson slot bot\\.\n\n" +
			"Group admins can run /setup to configure lesson days and time slots\\.\n" +
			"Send /help for all commands\\.",

		LanguageChanged: "✅ Language set to English\\.",

		DayNames: map[string]string{
			"monday": "Monday", "tuesday": "Tuesday", "wednesday": "Wednesday",
			"thursday": "Thursday", "friday": "Friday", "saturday": "Saturday", "sunday": "Sunday",
		},
	},

	LangUZ: {
		PickLanguage: "🌐 Iltimos, tilingizni tanlang:",

		AdminOnly:      "🚫 Bu buyruqdan faqat guruh adminlari foydalana oladi.",
		SetupCancelled: "❌ Sozlash bekor qilindi.",
		NoActiveSetup:  "ℹ️ Bekor qilish uchun faol sozlash yo'q.",

		HelpText: "📚 *Dars Slot Boti*\n\n" +
			"Meni guruhingizga qo'shing va admin qiling — chat va adminlarni avtomatik aniqlayman\\.\n\n" +
			"*Hammaga:*\n" +
			"/language — Tilni o'zgartirish\n" +
			"/status — Joriy sozlamalar va bronlarni ko'rish\n" +
			"/help — Ushbu xabarni ko'rsatish\n\n" +
			"*Faqat guruh adminlari:*\n" +
			"/setup — Dars kunlari va vaqtlarini sozlash\n" +
			"/broadcast — Bron klaviaturasini hozir yuborish \\(test\\)\n" +
			"/cancel — Joriy sozlashni bekor qilish\n\n" +
			"_Bron klaviaturalari har bir dars kunidan oldin soat 18:00 da yuboriladi\\._",

		StatusTitle:      "📊 *Bot holati*",
		StatusGroup:      "*Guruh chat ID:* %s",
		StatusDays:       "*Dars kunlari:* %s",
		StatusSlots:      "*Vaqt slotlari:* %s",
		StatusBookings:   "*Joriy bronlar:*\n%s",
		StatusNoBookings: "_Hali yo'q_",
		StatusNoGroup:    "_Hali guruhda emas_",

		WizStep1Title:    "⚙️ *Dars sozlamalari — 1\\-qadam, 3 tadan*",
		WizStep1Body:     "Dars bo'ladigan kunlarni tanlang\\.\nBekor qilish uchun qayta bosing\\. Tayyor bo'lgach *Keyingi* tugmasini bosing\\.",
		WizDaysNextEmpty: "⚠️ Kamida bitta kun tanlang",
		WizDaysNext:      "Keyingi →",

		WizStep2Title: "⚙️ *Dars sozlamalari — 2\\-qadam, 3 tadan*",
		WizStep2Body: "Mavjud vaqt slotlarini bo'sh joy bilan yozing:\n\n" +
			"`08:00 09:00 10:00 11:00`\n\n" +
			"_Istalgan vaqtda bekor qilish uchun /cancel yuboring\\._",

		WizStep3Title: "⚙️ *Dars sozlamalari — 3\\-qadam, 3 tadan*",
		WizStep3Body: "Sozlamalaringizni tasdiqlang:\n\n" +
			"📅 *Dars kunlari:* %s\n" +
			"🕒 *Vaqt slotlari:* %s\n\n" +
			"Saqlash uchun *Qo'llash* tugmasini bosing\\.",
		WizApply:     "✅ Qo'llash",
		WizStartOver: "🔄 Qaytadan boshlash",
		WizApplied: "✅ *Sozlash tugadi\\!*\n\n" +
			"📅 Dars kunlari: *%s*\n" +
			"🕒 Vaqt slotlari: *%s*\n\n" +
			"_Bron klaviaturalari har bir dars kunidan oldin soat 18:00 da yuboriladi\\._",
		WizRestarted: "🔄 Sozlash qayta boshlandi\\. /setup buyrug'ini yuboring\\.",

		ErrBadTimeFormat: "❌ Noto'g'ri format `%s` — HH:MM ko'rinishida bo'lishi kerak",
		ErrInvalidTime:   "❌ Noto'g'ri vaqt `%s`",
		ErrNoSlots:       "❌ Hech qanday to'g'ri vaqt sloti kiritilmadi",
		ErrTryAgain:      "Qayta urinib ko'ring, masalan: `08:00 09:00 10:00`",

		BroadcastMsg: "📅 *Ertaga dars\\!* Vaqt slotingizni tanlang:",

		BookedAlert:  "✅ %s vaqti bron qilindi!",
		TakenAlert:   "❌ %s vaqti allaqachon band!",
		TooSlowAlert: "⚡ Kech qoldingiz! %s vaqti band bo'lib qoldi.",

		JoinMsg: "👋 Salom *%s*\\! Men dars slotlari botiман\\.\n\n" +
			"Guruh adminlari /setup buyrug'i orqali dars kunlari va vaqtlarini sozlay oladi\\.\n" +
			"Barcha buyruqlar uchun /help yuboring\\.",

		LanguageChanged: "✅ Til O'zbekchaga o'zgartirildi\\.",

		DayNames: map[string]string{
			"monday": "Dushanba", "tuesday": "Seshanba", "wednesday": "Chorshanba",
			"thursday": "Payshanba", "friday": "Juma", "saturday": "Shanba", "sunday": "Yakshanba",
		},
	},

	LangRU: {
		PickLanguage: "🌐 Пожалуйста, выберите язык:",

		AdminOnly:      "🚫 Эта команда доступна только администраторам группы.",
		SetupCancelled: "❌ Настройка отменена.",
		NoActiveSetup:  "ℹ️ Нет активной настройки для отмены.",

		HelpText: "📚 *Бот расписания уроков*\n\n" +
			"Добавьте меня в группу и сделайте администратором — я автоматически определю чат и админов\\.\n\n" +
			"*Для всех:*\n" +
			"/language — Изменить язык\n" +
			"/status — Показать настройки и бронирования\n" +
			"/help — Показать это сообщение\n\n" +
			"*Только для администраторов:*\n" +
			"/setup — Настроить дни и временные слоты\n" +
			"/broadcast — Отправить клавиатуру бронирования сейчас \\(тест\\)\n" +
			"/cancel — Отменить текущую настройку\n\n" +
			"_Клавиатуры бронирования отправляются в 18:00 накануне каждого учебного дня\\._",

		StatusTitle:      "📊 *Статус бота*",
		StatusGroup:      "*ID группового чата:* %s",
		StatusDays:       "*Учебные дни:* %s",
		StatusSlots:      "*Временные слоты:* %s",
		StatusBookings:   "*Текущие бронирования:*\n%s",
		StatusNoBookings: "_Пока нет_",
		StatusNoGroup:    "_Ещё не добавлен в группу_",

		WizStep1Title:    "⚙️ *Настройка — Шаг 1 из 3*",
		WizStep1Body:     "Нажмите на дни, когда проходят уроки\\.\nНажмите снова, чтобы отменить выбор\\. Нажмите *Далее*, когда закончите\\.",
		WizDaysNextEmpty: "⚠️ Выберите хотя бы один день",
		WizDaysNext:      "Далее →",

		WizStep2Title: "⚙️ *Настройка — Шаг 2 из 3*",
		WizStep2Body: "Введите доступные временные слоты через пробел:\n\n" +
			"`08:00 09:00 10:00 11:00`\n\n" +
			"_Отправьте /cancel для отмены в любой момент\\._",

		WizStep3Title: "⚙️ *Настройка — Шаг 3 из 3*",
		WizStep3Body: "Подтвердите настройки:\n\n" +
			"📅 *Учебные дни:* %s\n" +
			"🕒 *Временные слоты:* %s\n\n" +
			"Нажмите *Применить* для сохранения\\.",
		WizApply:     "✅ Применить",
		WizStartOver: "🔄 Начать заново",
		WizApplied: "✅ *Настройка завершена\\!*\n\n" +
			"📅 Учебные дни: *%s*\n" +
			"🕒 Временные слоты: *%s*\n\n" +
			"_Клавиатуры бронирования будут отправляться в 18:00 накануне каждого учебного дня\\._",
		WizRestarted: "🔄 Настройка перезапущена\\. Используйте /setup для начала\\.",

		ErrBadTimeFormat: "❌ Неверный формат `%s` — ожидается ЧЧ:ММ",
		ErrInvalidTime:   "❌ Неверное время `%s`",
		ErrNoSlots:       "❌ Не указано ни одного корректного временного слота",
		ErrTryAgain:      "Попробуйте снова, например: `08:00 09:00 10:00`",

		BroadcastMsg: "📅 *Завтра урок\\!* Выберите свой временной слот:",

		BookedAlert:  "✅ Слот %s забронирован!",
		TakenAlert:   "❌ Слот %s уже занят!",
		TooSlowAlert: "⚡ Не успели! Слот %s только что заняли.",

		JoinMsg: "👋 Привет, *%s*\\! Я бот для записи на уроки\\.\n\n" +
			"Администраторы группы могут запустить /setup для настройки дней и слотов\\.\n" +
			"Отправьте /help для просмотра всех команд\\.",

		LanguageChanged: "✅ Язык изменён на русский\\.",

		DayNames: map[string]string{
			"monday": "Понедельник", "tuesday": "Вторник", "wednesday": "Среда",
			"thursday": "Четверг", "friday": "Пятница", "saturday": "Суббота", "sunday": "Воскресенье",
		},
	},
}

// tr returns the Strings for a given language, falling back to English.
func tr(lang Lang) *Strings {
	if s, ok := translations[lang]; ok {
		return s
	}
	return translations[LangEN]
}

// ─── Wizard Steps ─────────────────────────────────────────────────────────────

type WizardStep int

const (
	StepIdle    WizardStep = iota
	StepLang               // user hasn't picked a language yet
	StepDays               // picking lesson days
	StepHours              // typing time slots
	StepConfirm            // reviewing summary
)

type WizardSession struct {
	Step       WizardStep
	DraftDays  map[string]bool
	DraftSlots []string
	DaysMsgID  int // ID of the day-picker message so we can edit it in-place
}

// ─── Core State ───────────────────────────────────────────────────────────────

type LessonConfig struct {
	ActiveDays map[string]bool
	TimeSlots  []string
}

type State struct {
	mu sync.Mutex

	GroupChatID int64

	Config    LessonConfig
	Bookings  map[string]int64 // slot -> userID (0 = free)
	UserNames map[int64]string // userID -> display name
	UserLangs map[int64]Lang   // userID -> chosen language

	LastBroadcastMsgID int

	Wizards map[int64]*WizardSession // one session per admin
}

func newState() *State {
	return &State{
		Config: LessonConfig{
			ActiveDays: map[string]bool{"tuesday": true, "thursday": true},
			TimeSlots:  []string{"08:00", "09:00", "10:00", "11:00"},
		},
		Bookings:  make(map[string]int64),
		UserNames: make(map[int64]string),
		UserLangs: make(map[int64]Lang),
		Wizards:   make(map[int64]*WizardSession),
	}
}

// userLang returns the language for a user, defaulting to English.
func (s *State) userLang(userID int64) Lang {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.UserLangs[userID]; ok {
		return l
	}
	return LangEN
}

// hasChosenLang reports whether the user has ever picked a language.
func (s *State) hasChosenLang(userID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.UserLangs[userID]
	return ok
}

// ─── Weekday helpers ──────────────────────────────────────────────────────────

var orderedDays = []string{
	"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday",
}

// ─── Admin check ──────────────────────────────────────────────────────────────

func isGroupAdmin(bot *tgbotapi.BotAPI, s *State, userID int64) bool {
	s.mu.Lock()
	chatID := s.GroupChatID
	s.mu.Unlock()
	if chatID == 0 {
		return false
	}
	cfg := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: userID},
	}
	m, err := bot.GetChatMember(cfg)
	if err != nil {
		log.Printf("getChatMember error (userID=%d): %v", userID, err)
		return false
	}
	return m.Status == "administrator" || m.Status == "creator"
}

// ─── Language picker ──────────────────────────────────────────────────────────

func languageKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇬🇧 English", "lang_en"),
			tgbotapi.NewInlineKeyboardButtonData("🇺🇿 O'zbekcha", "lang_uz"),
			tgbotapi.NewInlineKeyboardButtonData("🇷🇺 Русский", "lang_ru"),
		),
	)
}

// sendLanguagePicker sends the language selection message. Uses a neutral
// trilingual prompt so the user understands it regardless of their language.
func sendLanguagePicker(bot *tgbotapi.BotAPI, chatID int64) {
	// Show the prompt in all three languages so it's universally understood.
	text := "🌐 Please choose your language / Iltimos, tilni tanlang / Пожалуйста, выберите язык:"
	m := tgbotapi.NewMessage(chatID, escMD(text))
	m.ParseMode = "MarkdownV2"
	m.ReplyMarkup = languageKeyboard()
	if _, err := bot.Send(m); err != nil {
		log.Printf("sendLanguagePicker error: %v", err)
	}
}

// handleLangCallback processes "lang_*" callbacks.
// Returns true if it consumed the callback.
func handleLangCallback(bot *tgbotapi.BotAPI, s *State, cq *tgbotapi.CallbackQuery) bool {
	data := cq.Data
	if !strings.HasPrefix(data, "lang_") {
		return false
	}

	bot.Request(tgbotapi.NewCallback(cq.ID, "")) //nolint:errcheck

	userID := cq.From.ID
	chatID := cq.Message.Chat.ID

	var chosen Lang
	switch data {
	case "lang_en":
		chosen = LangEN
	case "lang_uz":
		chosen = LangUZ
	case "lang_ru":
		chosen = LangRU
	default:
		return true
	}

	s.mu.Lock()
	s.UserLangs[userID] = chosen

	// If the user was mid-wizard waiting for language, advance them.
	session, inWizard := s.Wizards[userID]
	if inWizard && session.Step == StepLang {
		session.Step = StepDays
	}
	s.mu.Unlock()

	lang := tr(chosen)
	replyTo(bot, chatID, lang.LanguageChanged)

	// If they were in the wizard, continue to step 1.
	if inWizard {
		startDayStep(bot, s, chatID, userID)
	}

	return true
}

// ─── Group registration ───────────────────────────────────────────────────────

func handleMyChatMember(bot *tgbotapi.BotAPI, s *State, update tgbotapi.Update) {
	mcm := update.MyChatMember
	if mcm == nil {
		return
	}
	chat := mcm.Chat
	if chat.Type != "group" && chat.Type != "supergroup" {
		return
	}
	switch mcm.NewChatMember.Status {
	case "member", "administrator":
		s.mu.Lock()
		prev := s.GroupChatID
		s.GroupChatID = chat.ID
		s.mu.Unlock()
		if prev != chat.ID {
			log.Printf("Bot added to group %q (chatID=%d)", chat.Title, chat.ID)
			// Welcome in English by default; admins will set their own language via /setup.
			replyTo(bot, chat.ID, fmt.Sprintf(tr(LangEN).JoinMsg, escMD(chat.Title)))
		}
	case "left", "kicked":
		s.mu.Lock()
		if s.GroupChatID == chat.ID {
			s.GroupChatID = 0
		}
		s.mu.Unlock()
	}
}

// ─── Wizard: Step 1 — Day picker ─────────────────────────────────────────────

func dayPickerKeyboard(draft map[string]bool, lang Lang) tgbotapi.InlineKeyboardMarkup {
	names := tr(lang).DayNames
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(orderedDays); i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		for j := i; j < i+2 && j < len(orderedDays); j++ {
			d := orderedDays[j]
			label := names[d]
			if draft[d] {
				label = "✅ " + label
			}
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(label, "wiz_day_"+d))
		}
		rows = append(rows, row)
	}
	nextLabel := tr(lang).WizDaysNext
	if len(draft) == 0 {
		nextLabel = tr(lang).WizDaysNextEmpty
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(nextLabel, "wiz_days_next"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// startDayStep sends the day-picker message and records its ID.
func startDayStep(bot *tgbotapi.BotAPI, s *State, chatID, userID int64) {
	lang := s.userLang(userID)
	l := tr(lang)

	s.mu.Lock()
	session, ok := s.Wizards[userID]
	if !ok {
		session = &WizardSession{Step: StepDays, DraftDays: make(map[string]bool)}
		s.Wizards[userID] = session
	}
	session.Step = StepDays
	s.mu.Unlock()

	kb := dayPickerKeyboard(session.DraftDays, lang)
	m := tgbotapi.NewMessage(chatID, l.WizStep1Title+"\n\n"+l.WizStep1Body)
	m.ParseMode = "MarkdownV2"
	m.ReplyMarkup = kb
	sent, err := bot.Send(m)
	if err != nil {
		log.Printf("startDayStep send error: %v", err)
		return
	}
	s.mu.Lock()
	session.DaysMsgID = sent.MessageID
	s.mu.Unlock()
}

// startWizard is the entry point. Shows language picker first if needed.
func startWizard(bot *tgbotapi.BotAPI, s *State, chatID, userID int64) {
	if !s.hasChosenLang(userID) {
		// Create a waiting session and show the language picker.
		s.mu.Lock()
		s.Wizards[userID] = &WizardSession{Step: StepLang, DraftDays: make(map[string]bool)}
		s.mu.Unlock()
		sendLanguagePicker(bot, chatID)
		return
	}
	startDayStep(bot, s, chatID, userID)
}

// ─── Wizard: Callback handler ─────────────────────────────────────────────────

// handleWizardCallback processes all "wiz_*" callbacks.
// Returns true if consumed.
func handleWizardCallback(bot *tgbotapi.BotAPI, s *State, cq *tgbotapi.CallbackQuery) bool {
	data := cq.Data
	if !strings.HasPrefix(data, "wiz_") {
		return false
	}

	bot.Request(tgbotapi.NewCallback(cq.ID, "")) //nolint:errcheck

	userID := cq.From.ID
	chatID := cq.Message.Chat.ID
	lang := s.userLang(userID)
	l := tr(lang)

	s.mu.Lock()
	session, active := s.Wizards[userID]
	s.mu.Unlock()
	if !active {
		return true // stale button
	}

	switch {
	// ── Toggle a day ──────────────────────────────────────────────────────────
	case strings.HasPrefix(data, "wiz_day_"):
		day := strings.TrimPrefix(data, "wiz_day_")
		s.mu.Lock()
		if session.DraftDays[day] {
			delete(session.DraftDays, day)
		} else {
			session.DraftDays[day] = true
		}
		draft := make(map[string]bool, len(session.DraftDays))
		for k, v := range session.DraftDays {
			draft[k] = v
		}
		msgID := session.DaysMsgID
		s.mu.Unlock()

		edit := tgbotapi.NewEditMessageReplyMarkup(chatID, msgID, dayPickerKeyboard(draft, lang))
		if _, err := bot.Send(edit); err != nil {
			if !strings.Contains(err.Error(), "message is not modified") {
				log.Printf("day toggle edit error: %v", err)
			}
		}

	// ── Next: days → hours ────────────────────────────────────────────────────
	case data == "wiz_days_next":
		s.mu.Lock()
		nDays := len(session.DraftDays)
		s.mu.Unlock()
		if nDays == 0 {
			bot.Request(tgbotapi.CallbackConfig{ //nolint:errcheck
				CallbackQueryID: cq.ID,
				Text:            l.WizDaysNextEmpty,
				ShowAlert:       true,
			})
			return true
		}
		s.mu.Lock()
		session.Step = StepHours
		s.mu.Unlock()
		replyTo(bot, chatID, l.WizStep2Title+"\n\n"+l.WizStep2Body)

	// ── Apply ─────────────────────────────────────────────────────────────────
	case data == "wiz_confirm_apply":
		s.mu.Lock()
		if session.Step != StepConfirm {
			s.mu.Unlock()
			return true
		}
		days := make(map[string]bool, len(session.DraftDays))
		for k, v := range session.DraftDays {
			days[k] = v
		}
		slots := make([]string, len(session.DraftSlots))
		copy(slots, session.DraftSlots)
		s.Config.ActiveDays = days
		s.Config.TimeSlots = slots
		s.Bookings = make(map[string]int64)
		for _, sl := range slots {
			s.Bookings[sl] = 0
		}
		delete(s.Wizards, userID)
		s.mu.Unlock()

		dayList := localDayList(days, lang)
		replyTo(bot, chatID, fmt.Sprintf(l.WizApplied,
			escMD(strings.Join(dayList, ", ")),
			escMD(strings.Join(slots, ", ")),
		))

	// ── Start over ────────────────────────────────────────────────────────────
	case data == "wiz_confirm_restart":
		s.mu.Lock()
		delete(s.Wizards, userID)
		s.mu.Unlock()
		replyTo(bot, chatID, l.WizRestarted)
	}

	return true
}

// ─── Wizard: text input (Step 2 — hours) ─────────────────────────────────────

// handleWizardText intercepts free-text and /cancel during an active session.
// Returns true if consumed.
func handleWizardText(bot *tgbotapi.BotAPI, s *State, msg *tgbotapi.Message) bool {
	userID := msg.From.ID
	chatID := msg.Chat.ID

	s.mu.Lock()
	session, active := s.Wizards[userID]
	s.mu.Unlock()
	if !active {
		return false
	}

	lang := s.userLang(userID)
	l := tr(lang)

	// /cancel works at any wizard step.
	if msg.IsCommand() && msg.Command() == "cancel" {
		s.mu.Lock()
		delete(s.Wizards, userID)
		s.mu.Unlock()
		replyTo(bot, chatID, l.SetupCancelled)
		return true
	}

	// Only consume plain text during StepHours.
	if session.Step != StepHours || msg.IsCommand() {
		return false
	}

	args := strings.Fields(msg.Text)
	slots, err := parseSlots(args)
	if err != nil {
		replyTo(bot, chatID, escMD(err.Error())+"\n"+l.ErrTryAgain)
		return true
	}

	s.mu.Lock()
	session.DraftSlots = slots
	session.Step = StepConfirm
	draft := make(map[string]bool, len(session.DraftDays))
	for k, v := range session.DraftDays {
		draft[k] = v
	}
	s.mu.Unlock()

	dayList := localDayList(draft, lang)
	text := fmt.Sprintf(l.WizStep3Title+"\n\n"+l.WizStep3Body,
		escMD(strings.Join(dayList, ", ")),
		escMD(strings.Join(slots, ", ")),
	)
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = "MarkdownV2"
	m.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(l.WizApply, "wiz_confirm_apply"),
			tgbotapi.NewInlineKeyboardButtonData(l.WizStartOver, "wiz_confirm_restart"),
		),
	)
	if _, err := bot.Send(m); err != nil {
		log.Printf("sendConfirmation error: %v", err)
	}
	return true
}

// ─── Booking keyboard ─────────────────────────────────────────────────────────

func buildKeyboard(s *State) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, slot := range s.Config.TimeSlots {
		uid := s.Bookings[slot]
		var label, data string
		if uid == 0 {
			label = "🕒 " + slot
			data = "book_" + slot
		} else {
			name := s.UserNames[uid]
			if name == "" {
				name = strconv.FormatInt(uid, 10)
			}
			label = fmt.Sprintf("❌ %s (%s)", slot, name)
			data = "taken_" + slot
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, data),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// ─── Broadcast ────────────────────────────────────────────────────────────────

func broadcastBookingMessage(bot *tgbotapi.BotAPI, s *State) {
	s.mu.Lock()
	chatID := s.GroupChatID
	if chatID == 0 {
		s.mu.Unlock()
		log.Println("Broadcast skipped: not in a group yet.")
		return
	}
	s.Bookings = make(map[string]int64)
	for _, sl := range s.Config.TimeSlots {
		s.Bookings[sl] = 0
	}
	kb := buildKeyboard(s)
	s.mu.Unlock()

	// Broadcast is always in all three languages so every student understands.
	text := tr(LangEN).BroadcastMsg + "\n" + tr(LangUZ).BroadcastMsg + "\n" + tr(LangRU).BroadcastMsg
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = "MarkdownV2"
	m.ReplyMarkup = kb
	sent, err := bot.Send(m)
	if err != nil {
		log.Printf("broadcast error: %v", err)
		return
	}
	s.mu.Lock()
	s.LastBroadcastMsgID = sent.MessageID
	s.mu.Unlock()
	log.Printf("Broadcast sent to chatID=%d, messageID=%d", chatID, sent.MessageID)
}

func editBroadcastKeyboard(bot *tgbotapi.BotAPI, s *State) {
	s.mu.Lock()
	chatID := s.GroupChatID
	msgID := s.LastBroadcastMsgID
	kb := buildKeyboard(s)
	s.mu.Unlock()
	if chatID == 0 || msgID == 0 {
		return
	}
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, msgID, kb)
	if _, err := bot.Send(edit); err != nil {
		if !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("edit keyboard error: %v", err)
		}
	}
}

// ─── Scheduler ────────────────────────────────────────────────────────────────

func startScheduler(bot *tgbotapi.BotAPI, s *State) {
	go func() {
		now := time.Now()
		time.Sleep(time.Duration(60-now.Second())*time.Second - time.Duration(now.Nanosecond()))
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for t := range ticker.C {
			if t.Hour() != 18 || t.Minute() != 0 {
				continue
			}
			tomorrow := t.AddDate(0, 0, 1)
			day := strings.ToLower(tomorrow.Weekday().String())
			s.mu.Lock()
			isLesson := s.Config.ActiveDays[day]
			s.mu.Unlock()
			if isLesson {
				log.Printf("Scheduler: %s is a lesson day — broadcasting", day)
				broadcastBookingMessage(bot, s)
			}
		}
	}()
}

// ─── Booking callback ─────────────────────────────────────────────────────────

func handleBookingCallback(bot *tgbotapi.BotAPI, s *State, cq *tgbotapi.CallbackQuery) {
	ack := tgbotapi.NewCallback(cq.ID, "")
	userID := cq.From.ID
	data := cq.Data
	lang := s.userLang(userID)
	l := tr(lang)

	s.mu.Lock()
	if cq.From.UserName != "" {
		s.UserNames[userID] = "@" + cq.From.UserName
	} else {
		s.UserNames[userID] = cq.From.FirstName
	}
	s.mu.Unlock()

	switch {
	case strings.HasPrefix(data, "taken_"):
		slot := strings.TrimPrefix(data, "taken_")
		ack.Text = fmt.Sprintf(l.TakenAlert, slot)
		ack.ShowAlert = true
		bot.Request(ack) //nolint:errcheck

	case strings.HasPrefix(data, "book_"):
		slot := strings.TrimPrefix(data, "book_")
		s.mu.Lock()
		if existing := s.Bookings[slot]; existing != 0 && existing != userID {
			s.mu.Unlock()
			ack.Text = fmt.Sprintf(l.TooSlowAlert, slot)
			ack.ShowAlert = true
			bot.Request(ack) //nolint:errcheck
			return
		}
		for existingSlot, uid := range s.Bookings {
			if uid == userID && existingSlot != slot {
				s.Bookings[existingSlot] = 0
				break
			}
		}
		s.Bookings[slot] = userID
		s.mu.Unlock()
		ack.Text = fmt.Sprintf(l.BookedAlert, slot)
		bot.Request(ack) //nolint:errcheck
		editBroadcastKeyboard(bot, s)

	default:
		bot.Request(ack) //nolint:errcheck
	}
}

// ─── Status & Help ────────────────────────────────────────────────────────────

func handleStatus(bot *tgbotapi.BotAPI, s *State, chatID int64, userID int64) {
	lang := s.userLang(userID)
	l := tr(lang)

	s.mu.Lock()
	groupChatID := s.GroupChatID
	dayList := localDayList(s.Config.ActiveDays, lang)
	slots := make([]string, len(s.Config.TimeSlots))
	copy(slots, s.Config.TimeSlots)
	var booked []string
	for _, sl := range s.Config.TimeSlots {
		if uid := s.Bookings[sl]; uid != 0 {
			booked = append(booked, fmt.Sprintf("  • %s → %s", sl, s.UserNames[uid]))
		}
	}
	s.mu.Unlock()

	groupInfo := l.StatusNoGroup
	if groupChatID != 0 {
		groupInfo = fmt.Sprintf("`%d`", groupChatID)
	}
	bookingText := l.StatusNoBookings
	if len(booked) > 0 {
		bookingText = strings.Join(booked, "\n")
	}

	replyTo(bot, chatID, l.StatusTitle+"\n\n"+
		fmt.Sprintf(l.StatusGroup, groupInfo)+"\n"+
		fmt.Sprintf(l.StatusDays, escMD(strings.Join(dayList, ", ")))+"\n"+
		fmt.Sprintf(l.StatusSlots, escMD(strings.Join(slots, ", ")))+"\n\n"+
		fmt.Sprintf(l.StatusBookings, bookingText),
	)
}

func handleHelp(bot *tgbotapi.BotAPI, s *State, chatID int64, userID int64) {
	replyTo(bot, chatID, tr(s.userLang(userID)).HelpText)
}

// ─── /language command ────────────────────────────────────────────────────────

func handleLanguageCommand(bot *tgbotapi.BotAPI, chatID int64) {
	sendLanguagePicker(bot, chatID)
}

// ─── Utilities ────────────────────────────────────────────────────────────────

// replyTo sends a MarkdownV2 message. Used for all bot-initiated messages.
func replyTo(bot *tgbotapi.BotAPI, chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = "MarkdownV2"
	if _, err := bot.Send(m); err != nil {
		log.Printf("replyTo error (chatID=%d): %v", chatID, err)
	}
}

// escMD escapes all MarkdownV2 special characters.
func escMD(s string) string {
	special := `\_*[]()~` + "`" + `>#+-=|{}.!`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(special, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// localDayList returns day names in the given language, in week order.
func localDayList(days map[string]bool, lang Lang) []string {
	names := tr(lang).DayNames
	var list []string
	for _, d := range orderedDays {
		if days[d] {
			list = append(list, names[d])
		}
	}
	return list
}

// parseSlots validates and normalises HH:MM strings.
func parseSlots(args []string) ([]string, error) {
	var slots []string
	seen := map[string]bool{}
	for _, a := range args {
		parts := strings.Split(a, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad format `%s` — expected HH:MM", a)
		}
		h, errH := strconv.Atoi(parts[0])
		mn, errM := strconv.Atoi(parts[1])
		if errH != nil || errM != nil || h < 0 || h > 23 || mn < 0 || mn > 59 {
			return nil, fmt.Errorf("invalid time `%s`", a)
		}
		norm := fmt.Sprintf("%02d:%02d", h, mn)
		if !seen[norm] {
			seen[norm] = true
			slots = append(slots, norm)
		}
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("no valid time slots provided")
	}
	return slots, nil
}

// recordName caches the display name for a user.
func recordName(s *State, userID int64, username, firstName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if username != "" {
		s.UserNames[userID] = "@" + username
	} else {
		s.UserNames[userID] = firstName
	}
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN environment variable is not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("failed to connect to Telegram: %v", err)
	}
	log.Printf("Authorised as @%s", bot.Self.UserName)

	state := newState()
	startScheduler(bot, state)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	u.AllowedUpdates = []string{"message", "callback_query", "my_chat_member"}
	updates := bot.GetUpdatesChan(u)

	for update := range updates {

		// ── Bot membership change ─────────────────────────────────────────────
		if update.MyChatMember != nil {
			handleMyChatMember(bot, state, update)
			continue
		}

		// ── Callback queries ──────────────────────────────────────────────────
		if cq := update.CallbackQuery; cq != nil {
			recordName(state, cq.From.ID, cq.From.UserName, cq.From.FirstName)
			switch {
			case handleLangCallback(bot, state, cq): // "lang_*"
			case handleWizardCallback(bot, state, cq): // "wiz_*"
			default:
				go handleBookingCallback(bot, state, cq) // "book_*" / "taken_*"
			}
			continue
		}

		// ── Text messages ─────────────────────────────────────────────────────
		msg := update.Message
		if msg == nil {
			continue
		}

		userID := msg.From.ID
		chatID := msg.Chat.ID
		recordName(state, userID, msg.From.UserName, msg.From.FirstName)

		// If the user hasn't chosen a language yet and this isn't /start or
		// /language, prompt them first (but only in private / direct messages
		// to avoid flooding a group with picker messages for every new member).
		if !state.hasChosenLang(userID) &&
			!(msg.IsCommand() && (msg.Command() == "start" || msg.Command() == "language")) {
			if msg.Chat.IsPrivate() {
				sendLanguagePicker(bot, chatID)
				continue
			}
		}

		// Wizard text handler takes priority (handles /cancel too).
		if handleWizardText(bot, state, msg) {
			continue
		}

		if !msg.IsCommand() {
			continue
		}

		lang := state.userLang(userID)
		l := tr(lang)

		switch msg.Command() {
		case "start", "help":
			// On very first contact, show language picker before help.
			if !state.hasChosenLang(userID) {
				sendLanguagePicker(bot, chatID)
			} else {
				handleHelp(bot, state, chatID, userID)
			}

		case "language":
			handleLanguageCommand(bot, chatID)

		case "setup":
			if !isGroupAdmin(bot, state, userID) {
				replyTo(bot, chatID, l.AdminOnly)
				continue
			}
			startWizard(bot, state, chatID, userID)

		case "broadcast":
			if !isGroupAdmin(bot, state, userID) {
				replyTo(bot, chatID, l.AdminOnly)
				continue
			}
			go broadcastBookingMessage(bot, state)

		case "cancel":
			state.mu.Lock()
			_, active := state.Wizards[userID]
			if active {
				delete(state.Wizards, userID)
			}
			state.mu.Unlock()
			if active {
				replyTo(bot, chatID, l.SetupCancelled)
			} else {
				replyTo(bot, chatID, l.NoActiveSetup)
			}

		case "status":
			handleStatus(bot, state, chatID, userID)
		}
	}
}
