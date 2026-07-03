// Package archtest машинно проверяет dependency rule из README:
//
//	domain — ничего не импортирует, кроме stdlib;
//	app    — только domain (+stdlib) и подпакеты СВОЕЙ фичи
//	         (вертикальный слайс владеет своими подпакетами: например,
//	         access/accesstest — контрактный сьют портов access).
//
// «Нарушение dependency rule = архитектура сломана» — поэтому это тест,
// а не пункт code-review. Adapter/platform/cmd не проверяем: их свобода
// (adapter импортирует app+domain+другие адаптеры) правилом не ограничена.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/tumour/openwrt-bot"

func TestDomain_ImportsOnlyStdlib(t *testing.T) {
	forEachImport(t, "../domain", func(file, imp string) {
		if !isStdlib(imp) {
			t.Errorf("%s: импорт %q — domain должен зависеть только от stdlib", file, imp)
		}
	})
}

func TestApp_ImportsOnlyDomainAndStdlib(t *testing.T) {
	forEachImport(t, "../app", func(file, imp string) {
		if isStdlib(imp) || imp == module+"/internal/domain" {
			return
		}
		if feat := featureOf(file); feat != "" && sameFeature(imp, feat) {
			return
		}
		t.Errorf("%s: импорт %q — app может зависеть только от domain, stdlib и подпакетов своей фичи", file, imp)
	})
}

// featureOf извлекает имя фичи из пути ../app/<фича>/…
func featureOf(file string) string {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for i, p := range parts {
		if p == "app" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// sameFeature — импорт лежит внутри того же вертикального слайса.
func sameFeature(imp, feature string) bool {
	prefix := module + "/internal/app/" + feature
	return imp == prefix || strings.HasPrefix(imp, prefix+"/")
}

// isStdlib: у stdlib-пакетов первый сегмент пути без точки ("fmt", "net/http"),
// у внешних — домен ("github.com/...", "gopkg.in/..."). Эвристика официальная,
// её использует и go-toolchain (см. cmd/go/internal/load).
func isStdlib(imp string) bool {
	return !strings.Contains(strings.Split(imp, "/")[0], ".")
}

// forEachImport обходит все .go-файлы под root (включая _test.go — тесты слоёв
// подчиняются тем же правилам) и зовёт check на каждый импорт.
func forEachImport(t *testing.T, root string, check func(file, imp string)) {
	t.Helper()
	fset := token.NewFileSet()
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			check(path, strings.Trim(imp.Path.Value, `"`))
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk %s: %v", root, walkErr)
	}
}
