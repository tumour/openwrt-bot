package telegram

import (
	"testing"

	"github.com/tumour/openwrt-bot/internal/app/access"
	"github.com/tumour/openwrt-bot/internal/domain"
)

func grantWith(perms ...domain.Permission) access.Grant {
	return access.Grant{
		User: domain.NewActiveUser(1, "x", "r"),
		Role: domain.Role{Name: "r", Permissions: perms},
	}
}

func adminGrant() access.Grant { return grantWith(domain.AllPermissions()...) }

// TestMenuCommandsFor_Valid проверяет, что пункты меню удовлетворяют
// ограничениям Telegram BotCommand и что алиасы (inMenu=false) в меню
// не попадают. Handlers{} достаточно: читаются только имена/описания и
// method-values (создаются на nil-указателях без паники — вызова нет).
func TestMenuCommandsFor_Valid(t *testing.T) {
	menu := menuCommandsFor(Handlers{}, adminGrant())
	if len(menu) == 0 {
		t.Fatal("меню команд пустое")
	}
	// MAC-команды спрятаны из меню (функционал — в карточках /list).
	hidden := map[string]bool{"start": true, "ban": true, "unban": true, "vpnoff": true, "vpnon": true}
	for _, c := range menu {
		if hidden[c.Text] {
			t.Errorf("/%s не должен попадать в меню", c.Text)
		}
		// Имя: 1-32 символа, только [a-z0-9_].
		if n := len(c.Text); n < 1 || n > 32 {
			t.Errorf("команда %q: длина имени %d, нужно 1-32", c.Text, n)
		}
		for _, r := range c.Text {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				t.Errorf("команда %q: недопустимый символ %q (только a-z, 0-9, _)", c.Text, r)
			}
		}
		// Описание: 3-256 символов (считаем руны, не байты — описания кириллические).
		if n := len([]rune(c.Description)); n < 3 || n > 256 {
			t.Errorf("команда %q: описание %d символов, нужно 3-256", c.Text, n)
		}
	}
}

// «Нет пермишена = нет кнопки»: меню и клавиатура строятся по правам.
func TestVisibility_FollowsPermissions(t *testing.T) {
	userGrant := grantWith(domain.PermViewStatus, domain.PermListDevices, domain.PermRunSpeedtest)

	for _, c := range menuCommandsFor(Handlers{}, userGrant) {
		if c.Text == "users" {
			t.Error("/users не должен быть в меню без manage_users")
		}
	}
	found := false
	for _, c := range menuCommandsFor(Handlers{}, adminGrant()) {
		if c.Text == "users" {
			found = true
		}
	}
	if !found {
		t.Error("/users должен быть в меню админа")
	}

	countButtons := func(g access.Grant) int {
		n := 0
		for _, row := range mainKeyboard(Handlers{}, g).ReplyKeyboard {
			n += len(row)
		}
		return n
	}
	if got := countButtons(adminGrant()); got != 4 {
		t.Errorf("кнопок у админа = %d, want 4 (устройства/статус/спидтест/доступ)", got)
	}
	if got := countButtons(userGrant); got != 3 {
		t.Errorf("кнопок у юзера = %d, want 3 (без «Доступ»)", got)
	}
	if got := countButtons(grantWith()); got != 0 {
		t.Errorf("кнопок без прав = %d, want 0", got)
	}
}

// У каждой команды, кроме /start, объявлено право — забытая команда без
// права стала бы дырой (guard пропускает perm == "").
func TestCommands_AllHavePermissions(t *testing.T) {
	for _, c := range commands(Handlers{}) {
		if c.name != "start" && c.perm == "" {
			t.Errorf("/%s без права: каждая команда должна быть закрыта пермишеном", c.name)
		}
	}
}
