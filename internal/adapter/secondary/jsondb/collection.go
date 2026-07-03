// Package jsondb — generic-движок «JSON-файл как коллекция записей»:
// load/save целиком, конверт с версией схемы, атомарная запись через
// system.FileWriter. Движок не знает, ЧТО хранит: фичевые адаптеры
// (accessjson и будущие) объявляют свои файлы и записи сами — добавление
// новой сущности в проект не трогает ни движок, ни чужие адаптеры.
package jsondb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/tumour/openwrt-bot/internal/adapter/secondary/system"
)

// ErrSchemaVersion — версия файла не совпадает с ожидаемой адаптером.
// Фичевый адаптер поймает её, когда научится миграциям; пока это fail fast
// (например, бот откатили на файле от более новой версии).
var ErrSchemaVersion = errors.New("unsupported schema version")

// envelope — конверт файла: версия схемы + записи. Ключ items намеренно
// один для всех коллекций — формат приватен для движка.
type envelope[T any] struct {
	V     int `json:"v"`
	Items []T `json:"items"`
}

// Collection — одна коллекция записей T в одном файле. Потокобезопасность —
// забота владельца: фичевый адаптер держит один мьютекс на все свои
// коллекции, чтобы операции над связанными сущностями видели согласованное
// состояние.
type Collection[T any] struct {
	path    string
	version int
	reader  system.FileReader
	writer  system.FileWriter
}

func NewCollection[T any](path string, version int, r system.FileReader, w system.FileWriter) Collection[T] {
	return Collection[T]{path: path, version: version, reader: r, writer: w}
}

// Load читает все записи. Отсутствие файла — пустая коллекция (свежая
// установка); битый JSON или чужая версия схемы — ошибка: молча терять
// записи нельзя, fail fast честнее.
func (c Collection[T]) Load(ctx context.Context) ([]T, error) {
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
	if env.V != c.version {
		return nil, fmt.Errorf("%s: v%d, want v%d: %w", c.path, env.V, c.version, ErrSchemaVersion)
	}
	return env.Items, nil
}

// Save атомарно перезаписывает коллекцию целиком. Файл 600: данные не
// секрет уровня токена, но и не публичные.
func (c Collection[T]) Save(ctx context.Context, items []T) error {
	data, err := json.MarshalIndent(envelope[T]{V: c.version, Items: items}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", c.path, err)
	}
	return c.writer.WriteFileAtomic(ctx, c.path, append(data, '\n'), 0o600)
}
