package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"labspectra/internal/domain"
)

func TestRegistryReloadsOnExternalChange(t *testing.T) {
	p := NewPaths(t.TempDir())
	if err := os.MkdirAll(p.Backups(), 0o755); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(p)
	if err := r.Upsert(&domain.Analysis{ID: "IR-2026-001", Product: "A"}); err != nil {
		t.Fatal(err)
	}

	f, err := excelize.OpenFile(p.Registry())
	if err != nil {
		t.Fatal(err)
	}
	vals := []interface{}{"IR-2026-009", "", "снаружи"}
	if err := f.SetSheetRow(sheetName, "A3", &vals); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p.Registry(), future, future); err != nil {
		t.Fatal(err)
	}

	if err := r.Upsert(&domain.Analysis{ID: "IR-2026-002", Product: "B"}); err != nil {
		t.Fatal(err)
	}

	ids, err := r.IDs()
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	for _, want := range []string{"IR-2026-001", "IR-2026-002", "IR-2026-009"} {
		if !set[want] {
			t.Errorf("в реестре нет %s (внешняя правка потеряна): %v", want, ids)
		}
	}
}

func TestPruneSnapshots(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 15; i++ {
		n := filepath.Join(dir, "registry-"+fmt.Sprintf("%04d", i)+".xlsx")
		if err := os.WriteFile(n, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneSnapshots(dir, 10); err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "registry-*.xlsx"))
	if len(left) != 10 {
		t.Errorf("осталось %d снимков, ожидалось 10", len(left))
	}
	if _, err := os.Stat(filepath.Join(dir, "registry-0001.xlsx")); !os.IsNotExist(err) {
		t.Error("старейший снимок не удалён")
	}
	if _, err := os.Stat(filepath.Join(dir, "registry-0015.xlsx")); err != nil {
		t.Error("новейший снимок удалён по ошибке")
	}
}
