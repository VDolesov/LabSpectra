package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCharacteristicsMigratesProductCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "characteristics.json")
	old := []byte(`{"R2531":["pH","АК, ppm"],"V00S9":["pH","W, г/г"]}`)
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadCharacteristics(path, []string{"с.г., %"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"с.г., %", "pH", "АК, ppm", "W, г/г"} {
		if !containsString(got, want) {
			t.Fatalf("после миграции нет %q: %v", want, got)
		}
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
