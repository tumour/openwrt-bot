package presenter

import (
	"fmt"
	"html"
	"strings"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// AccessRequest — карточка заявки для админа (уведомление approve-flow).
func AccessRequest(u domain.User) string {
	return fmt.Sprintf("🔑 <b>Заявка на доступ</b>\n%s\nID: <code>%d</code>", userLabel(u), u.ID)
}

// RequestSent — ответ незнакомцу, подавшему заявку.
const RequestSent = "📨 Заявка отправлена — владелец бота решит, выдать ли доступ."

// RequestPending — повторный /start при живой заявке.
const RequestPending = "⏳ Заявка уже на рассмотрении."

// AccessGranted — уведомление одобренному.
const AccessGranted = "🎉 Доступ выдан. Нажмите /start — появится клавиатура управления."

// AccessDenied — уведомление отклонённому.
const AccessDenied = "🚫 В доступе отказано."

// UsersList — список пользователей для /users: заявки отдельным блоком,
// затем допущенные с ролями. Имена — из Telegram, поэтому экранируются.
func UsersList(users []domain.User) string {
	var pending, active []string
	for _, u := range users {
		switch u.Status {
		case domain.StatusPending:
			pending = append(pending, fmt.Sprintf("• %s — <code>%d</code>", userLabel(u), u.ID))
		default:
			active = append(active, fmt.Sprintf("• %s — <i>%s</i>", userLabel(u), html.EscapeString(string(u.Role))))
		}
	}
	var b strings.Builder
	b.WriteString("👥 <b>Доступ к боту</b>\n")
	if len(pending) > 0 {
		b.WriteString("\n⏳ <b>Заявки</b>\n")
		b.WriteString(strings.Join(pending, "\n"))
		b.WriteString("\n")
	}
	if len(active) > 0 {
		b.WriteString("\n✅ <b>Пользователи</b>\n")
		b.WriteString(strings.Join(active, "\n"))
		b.WriteString("\n")
	}
	if len(pending) == 0 && len(active) == 0 {
		b.WriteString("\nПусто.")
	}
	return b.String()
}

// RolePicker — заголовок выбора роли для пользователя.
func RolePicker(u domain.User) string {
	return fmt.Sprintf("🎭 Роль для %s (сейчас: <i>%s</i>):", userLabel(u), html.EscapeString(string(u.Role)))
}

// UserButtonLabel — подпись inline-кнопки пользователя (укороченная).
func UserButtonLabel(u domain.User) string {
	name := displayName(u)
	if r := []rune(name); len(r) > 24 {
		name = string(r[:23]) + "…"
	}
	return name
}

// userLabel — имя для текста сообщения (HTML-экранированное, жирное).
func userLabel(u domain.User) string {
	return "<b>" + html.EscapeString(displayName(u)) + "</b>"
}

func displayName(u domain.User) string {
	if strings.TrimSpace(u.Name) != "" {
		return u.Name
	}
	return fmt.Sprintf("id%d", u.ID)
}
