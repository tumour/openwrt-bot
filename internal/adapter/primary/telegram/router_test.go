package telegram

import "testing"

// TestMenuCommands_Valid проверяет, что пункты меню удовлетворяют ограничениям
// Telegram BotCommand и что алиасы (inMenu=false) в меню не попадают.
// Handlers{} достаточно: menuCommands читает только имена/описания и method-values
// (последние создаются на nil-указателях без паники — вызов тут не происходит).
func TestMenuCommands_Valid(t *testing.T) {
	menu := menuCommands(Handlers{})
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
