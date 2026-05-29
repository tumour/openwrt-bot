package system

import (
	"context"
	"fmt"
	"os"
)

// FileReader — отдельный порт для чтения файлов файловой системы. Сделан рядом
// с Runner, потому что оба находятся на одной границе "os-инфраструктура", но
// разные семантически: Runner запускает команды, FileReader читает файлы.
//
// Альтернатива — читать файлы через `cat` в Runner. Технически работает, но
// семантически путает (зачем форкать процесс ради чтения файла?), плюс ломается
// идемпотентность на больших файлах. Отдельный интерфейс честнее.
type FileReader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

// OSFileReader — реальная реализация через os.ReadFile. ctx сейчас не используется
// (os.ReadFile синхронный), но сигнатуру держим единообразной — на случай если
// в будущем заменим на stream-чтение с возможностью отмены.
type OSFileReader struct{}

func NewOSFileReader() *OSFileReader { return &OSFileReader{} }

func (r *OSFileReader) ReadFile(_ context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	return data, nil
}
