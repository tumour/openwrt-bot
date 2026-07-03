package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriter — порт записи файлов, пара к FileReader на той же границе
// «os-инфраструктура». Отдельный интерфейс: большинству адаптеров нужно
// только чтение, и давать им запись было бы нарушением ISP.
type FileWriter interface {
	// WriteFileAtomic пишет файл атомарно: tmp рядом с целью → fsync → rename.
	// Читатель никогда не увидит полузаписанный файл, а обрыв питания (роутер!)
	// оставит либо старую, либо новую версию целиком.
	WriteFileAtomic(ctx context.Context, path string, data []byte, perm os.FileMode) error
	MkdirAll(ctx context.Context, path string, perm os.FileMode) error
}

// OSFileWriter — реальная реализация поверх os. ctx в сигнатуре для
// единообразия границы (как у OSFileReader).
type OSFileWriter struct{}

func NewOSFileWriter() *OSFileWriter { return &OSFileWriter{} }

func (w *OSFileWriter) WriteFileAtomic(_ context.Context, path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create tmp for %s: %w", path, err)
	}
	defer os.Remove(tmp.Name()) // no-op после успешного rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write tmp for %s: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod tmp for %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fsync tmp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close tmp for %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename tmp to %s: %w", path, err)
	}
	return nil
}

func (w *OSFileWriter) MkdirAll(_ context.Context, path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	return nil
}
