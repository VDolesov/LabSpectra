package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

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
