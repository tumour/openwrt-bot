package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/middleware"
	"github.com/tumour/openwrt-bot/internal/adapter/primary/telegram/presenter"
	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
	tele "gopkg.in/telebot.v3"
)

// Access — управление доступом: заявка незнакомца (/start → approve-flow),
// уведомления админам с кнопками Разрешить/Отклонить и экран /users
// (заявки, роли, удаление). Требуемые права проверяет guard роутера,
// use cases перепроверяют актора сами (защита в глубину).
type Access struct {
	request   *access.RequestAccess
	approve   *access.Approve
	reject    *access.Reject
	listUsers *access.ListUsers
	listRoles *access.ListRoles
	setRole   *access.SetRole
	remove    *access.RemoveUser
}

func NewAccess(request *access.RequestAccess, approve *access.Approve, reject *access.Reject,
	listUsers *access.ListUsers, listRoles *access.ListRoles,
	setRole *access.SetRole, remove *access.RemoveUser) *Access {
	return &Access{
		request: request, approve: approve, reject: reject,
		listUsers: listUsers, listRoles: listRoles,
		setRole: setRole, remove: remove,
	}
}

// Уникальные id callback-кнопок approve-flow и экрана /users.
const (
	cbAccApprove = "acc_approve" // payload: id заявителя
	cbAccReject  = "acc_reject"  // payload: id заявителя
	cbAccUsers   = "acc_users"   // перерисовать список
	cbAccRole    = "acc_role"    // payload: id — открыть выбор роли
	cbAccSetRole = "acc_setrole" // payload: "id|role"
	cbAccRemove  = "acc_remove"  // payload: id
)

// RegisterCallbacks вешает обработчики inline-кнопок. guard — проверка права
// из роутера (та же, что у команд): у всех кнопок управления доступом —
// manage_users.
func (h *Access) RegisterCallbacks(bot *tele.Bot, guard func(domain.Permission, tele.HandlerFunc) tele.HandlerFunc) {
	manage := func(fn tele.HandlerFunc) tele.HandlerFunc { return guard(domain.PermManageUsers, fn) }
	bot.Handle(&tele.Btn{Unique: cbAccApprove}, manage(h.handleApprove))
	bot.Handle(&tele.Btn{Unique: cbAccReject}, manage(h.handleReject))
	bot.Handle(&tele.Btn{Unique: cbAccUsers}, manage(h.handleUsersRefresh))
	bot.Handle(&tele.Btn{Unique: cbAccRole}, manage(h.handleRolePicker))
	bot.Handle(&tele.Btn{Unique: cbAccSetRole}, manage(h.handleSetRole))
	bot.Handle(&tele.Btn{Unique: cbAccRemove}, manage(h.handleRemove))
}

// HandleRequest — /start от НЕдопущенного (единственный handler фичи, который
// зовётся без Grant — из Auth). Создаёт заявку и уведомляет админов кнопками.
func (h *Access) HandleRequest(c tele.Context) error {
	sender := c.Sender()
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.request.Execute(ctx, access.RequestAccessInput{
		UserID: sender.ID,
		Name:   senderName(sender),
	})
	if err != nil {
		_ = c.Send("⚠ Не получилось отправить заявку, попробуйте позже.")
		return fmt.Errorf("request access %d: %w", sender.ID, err)
	}
	if !out.Created {
		return c.Send(presenter.RequestPending)
	}

	// Уведомления админам — best-effort: заблокировавший бота админ не должен
	// сорвать заявку; ошибки собираем в лог (вернём наверх), юзеру — успех.
	var notifyErrs []error
	for _, admin := range out.Admins {
		_, err := c.Bot().Send(
			&tele.User{ID: int64(admin.ID)},
			presenter.AccessRequest(out.User),
			approveMarkup(out.User.ID),
			tele.ModeHTML,
		)
		if err != nil {
			notifyErrs = append(notifyErrs, fmt.Errorf("notify admin %d: %w", admin.ID, err))
		}
	}
	if err := c.Send(presenter.RequestSent); err != nil {
		notifyErrs = append(notifyErrs, err)
	}
	return errors.Join(notifyErrs...)
}

// HandleUsers — команда /users: список + кнопки управления.
func (h *Access) HandleUsers(c tele.Context) error {
	text, markup, err := h.usersScreen(c)
	if err != nil {
		_ = c.Send("⚠ Не удалось получить список пользователей.")
		return fmt.Errorf("/users: %w", err)
	}
	return c.Send(text, markup, tele.ModeHTML)
}

// --- callbacks ---

func (h *Access) handleApprove(c tele.Context) error {
	target, ok := callbackUserID(c)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ Битые данные кнопки"})
	}
	g, _ := middleware.GrantFrom(c)
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.approve.Execute(ctx, access.ApproveInput{ActorID: g.User.ID, TargetID: target})
	switch {
	case errors.Is(err, domain.ErrNotPending):
		return respondEdit(c, "Заявка уже обработана.", nil)
	case errors.Is(err, domain.ErrUserNotFound):
		return respondEdit(c, "Заявка отозвана или удалена.", nil)
	case err != nil:
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("approve %d: %w", target, err)
	}
	// Одобренному — приглашение (best-effort: мог заблокировать бота).
	_, _ = c.Bot().Send(&tele.User{ID: int64(out.User.ID)}, presenter.AccessGranted)
	return respondEdit(c, "✅ Доступ выдан: "+presenter.UserButtonLabel(out.User), &tele.CallbackResponse{Text: "✅ Выдан"})
}

func (h *Access) handleReject(c tele.Context) error {
	target, ok := callbackUserID(c)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ Битые данные кнопки"})
	}
	g, _ := middleware.GrantFrom(c)
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.reject.Execute(ctx, access.RejectInput{ActorID: g.User.ID, TargetID: target})
	switch {
	case errors.Is(err, domain.ErrNotPending):
		return respondEdit(c, "Заявка уже обработана.", nil)
	case errors.Is(err, domain.ErrUserNotFound):
		return respondEdit(c, "Заявка отозвана или удалена.", nil)
	case err != nil:
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("reject %d: %w", target, err)
	}
	_, _ = c.Bot().Send(&tele.User{ID: int64(out.User.ID)}, presenter.AccessDenied)
	return respondEdit(c, "🚫 Заявка отклонена: "+presenter.UserButtonLabel(out.User), &tele.CallbackResponse{Text: "🚫 Отклонена"})
}

// handleUsersRefresh — «Обновить» на экране /users.
func (h *Access) handleUsersRefresh(c tele.Context) error {
	text, markup, err := h.usersScreen(c)
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("users refresh: %w", err)
	}
	if err := editKeepalive(c, text, markup); err != nil {
		return err
	}
	return c.Respond()
}

// handleRolePicker — «🎭»: показать выбор роли для пользователя.
func (h *Access) handleRolePicker(c tele.Context) error {
	target, ok := callbackUserID(c)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ Битые данные кнопки"})
	}
	g, _ := middleware.GrantFrom(c)
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	users, err := h.listUsers.Execute(ctx, access.ListUsersInput{ActorID: g.User.ID})
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("role picker %d: %w", target, err)
	}
	var user domain.User
	found := false
	for _, u := range users.Users {
		if u.ID == target {
			user, found = u, true
			break
		}
	}
	if !found {
		return respondEdit(c, "Пользователь уже удалён.", nil)
	}
	roles, err := h.listRoles.Execute(ctx, access.ListRolesInput{ActorID: g.User.ID})
	if err != nil {
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("role picker %d: %w", target, err)
	}
	if err := editKeepalive(c, presenter.RolePicker(user), rolePickerMarkup(target, roles.Roles)); err != nil {
		return err
	}
	return c.Respond()
}

func (h *Access) handleSetRole(c tele.Context) error {
	target, role, ok := callbackUserRole(c)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ Битые данные кнопки"})
	}
	g, _ := middleware.GrantFrom(c)
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	_, err := h.setRole.Execute(ctx, access.SetRoleInput{ActorID: g.User.ID, TargetID: target, Role: role})
	switch {
	case errors.Is(err, domain.ErrLastAdmin):
		return c.Respond(&tele.CallbackResponse{Text: "⛔ Нельзя: последний админ"})
	case err != nil:
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("set role %d→%s: %w", target, role, err)
	}
	_ = c.Respond(&tele.CallbackResponse{Text: "🎭 Роль обновлена"})
	return h.handleUsersRefresh(c)
}

func (h *Access) handleRemove(c tele.Context) error {
	target, ok := callbackUserID(c)
	if !ok {
		return c.Respond(&tele.CallbackResponse{Text: "⚠ Битые данные кнопки"})
	}
	g, _ := middleware.GrantFrom(c)
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	_, err := h.remove.Execute(ctx, access.RemoveUserInput{ActorID: g.User.ID, TargetID: target})
	switch {
	case errors.Is(err, domain.ErrLastAdmin):
		return c.Respond(&tele.CallbackResponse{Text: "⛔ Нельзя: последний админ"})
	case errors.Is(err, domain.ErrUserNotFound):
		// уже удалён — просто перерисуем список
	case err != nil:
		_ = c.Respond(&tele.CallbackResponse{Text: "⚠ Не получилось"})
		return fmt.Errorf("remove %d: %w", target, err)
	}
	_ = c.Respond(&tele.CallbackResponse{Text: "🗑 Удалён"})
	return h.handleUsersRefresh(c)
}

// --- сборка экрана и разметок ---

// usersScreen собирает текст и кнопки /users по свежему состоянию.
func (h *Access) usersScreen(c tele.Context) (string, *tele.ReplyMarkup, error) {
	g, _ := middleware.GrantFrom(c)
	ctx, cancel := context.WithTimeout(middleware.BaseContext(c), handlerTimeout)
	defer cancel()

	out, err := h.listUsers.Execute(ctx, access.ListUsersInput{ActorID: g.User.ID})
	if err != nil {
		return "", nil, err
	}
	return presenter.UsersList(out.Users), usersMarkup(out.Users, g.User.ID), nil
}

// usersMarkup — по ряду кнопок на пользователя: заявке — принять/отклонить,
// активному — роль/удаление. Сам актор без кнопок: самоуправление ролью — путь
// к локауту, а самоудаление доступно другому админу.
func usersMarkup(users []domain.User, actor domain.UserID) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, u := range users {
		id := strconv.FormatInt(int64(u.ID), 10)
		label := presenter.UserButtonLabel(u)
		switch {
		case u.Status == domain.StatusPending:
			rows = append(rows, m.Row(
				m.Data("✅ "+label, cbAccApprove, id),
				m.Data("❌", cbAccReject, id),
			))
		case u.ID != actor:
			rows = append(rows, m.Row(
				m.Data("🎭 "+label, cbAccRole, id),
				m.Data("🗑", cbAccRemove, id),
			))
		}
	}
	rows = append(rows, m.Row(m.Data("🔄 Обновить", cbAccUsers)))
	m.Inline(rows...)
	return m
}

// approveMarkup — кнопки под уведомлением о заявке.
func approveMarkup(id domain.UserID) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	payload := strconv.FormatInt(int64(id), 10)
	m.Inline(m.Row(
		m.Data("✅ Разрешить", cbAccApprove, payload),
		m.Data("❌ Отклонить", cbAccReject, payload),
	))
	return m
}

// rolePickerMarkup — кнопка на роль + возврат к списку.
func rolePickerMarkup(target domain.UserID, roles []domain.Role) *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{}
	id := strconv.FormatInt(int64(target), 10)
	var rows []tele.Row
	for _, r := range roles {
		rows = append(rows, m.Row(m.Data("🎭 "+string(r.Name), cbAccSetRole, id+"|"+string(r.Name))))
	}
	rows = append(rows, m.Row(m.Data("⬅ К списку", cbAccUsers)))
	m.Inline(rows...)
	return m
}

// --- разбор callback-данных ---

func callbackUserID(c tele.Context) (domain.UserID, bool) {
	raw, err := strconv.ParseInt(strings.TrimSpace(c.Data()), 10, 64)
	if err != nil {
		return 0, false
	}
	id, err := domain.NewUserID(raw)
	return id, err == nil
}

func callbackUserRole(c tele.Context) (domain.UserID, domain.RoleName, bool) {
	target, rawRole, ok := strings.Cut(strings.TrimSpace(c.Data()), "|")
	if !ok {
		return 0, "", false
	}
	raw, err := strconv.ParseInt(target, 10, 64)
	if err != nil {
		return 0, "", false
	}
	id, err := domain.NewUserID(raw)
	if err != nil {
		return 0, "", false
	}
	role, err := domain.NewRoleName(rawRole)
	if err != nil {
		return 0, "", false
	}
	return id, role, true
}

// respondEdit — редактирование сообщения + toast одним движением.
func respondEdit(c tele.Context, text string, resp *tele.CallbackResponse) error {
	if err := editKeepalive(c, text, nil); err != nil {
		return err
	}
	if resp == nil {
		return c.Respond()
	}
	return c.Respond(resp)
}

// senderName — display-имя из Telegram: «Имя Фамилия (@username)».
func senderName(u *tele.User) string {
	name := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if u.Username != "" {
		if name == "" {
			return "@" + u.Username
		}
		return name + " (@" + u.Username + ")"
	}
	return name
}
