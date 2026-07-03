package accessstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
)

// schemaVersion — версия формата файлов стора. Меняется при несовместимой
// правке записей; load старой версии — место для миграции, более новой —
// ошибка (файл от следующей версии бота, откат без миграции запрещён).
const schemaVersion = 1

// errSchemaVersion — файл несовместимой (более новой) версии схемы.
var errSchemaVersion = errors.New("unsupported schema version")

// envelope — конверт файла: версия схемы + записи. Ключ items намеренно
// один для всех коллекций — формат приватен для адаптера.
type envelope[T any] struct {
	V     int `json:"v"`
	Items []T `json:"items"`
}

// collection — generic-движок «JSON-файл как коллекция записей»: load/save,
// версия схемы, атомарная запись. Общая механика обеих коллекций (users,
// roles) живёт здесь ОДИН раз; доменный смысл записей — в типизированных
// обёртках Store. Аналог «базовой модели» ORM, но приватный для адаптера:
// порты и use cases о нём не знают.
type collection[T any] struct {
	path   string
	reader system.FileReader
	writer system.FileWriter
}

// load читает все записи. Отсутствие файла — пустая коллекция (свежий бот),
// битый JSON или чужая версия схемы — ошибка: молча терять записи о доступе
// нельзя, fail fast честнее.
func (c *collection[T]) load(ctx context.Context) ([]T, error) {
	data, err := c.reader.ReadFile(ctx, c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var env envelope[T]
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse %s: %w", c.path, err)
	}
	if env.V != schemaVersion {
		// Место будущих миграций: case env.V == 1: migrateV1toV2(...)
		return nil, fmt.Errorf("%s: v%d: %w", c.path, env.V, errSchemaVersion)
	}
	return env.Items, nil
}

// save атомарно перезаписывает коллекцию целиком. Файл 600: список доступа —
// не секрет уровня токена, но и не публичные данные.
func (c *collection[T]) save(ctx context.Context, items []T) error {
	data, err := json.MarshalIndent(envelope[T]{V: schemaVersion, Items: items}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", c.path, err)
	}
	return c.writer.WriteFileAtomic(ctx, c.path, append(data, '\n'), 0o600)
}
