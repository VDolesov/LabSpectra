package index

import (
	"testing"

	"labspectra/internal/domain"
)

func a(id, product, batch, status string) *domain.Analysis {
	return &domain.Analysis{ID: id, Product: product, Batch: batch, Status: status}
}

func TestPutGetDelete(t *testing.T) {
	ix := New()
	ix.Put(a("IR-2026-001", "Этанол", "E1", "новый"))
	if got, ok := ix.Get("IR-2026-001"); !ok || got.Product != "Этанол" {
		t.Fatalf("Get вернул %v, %v", got, ok)
	}
	if ix.Len() != 1 {
		t.Fatalf("Len=%d, ожидалось 1", ix.Len())
	}
	ix.Delete("IR-2026-001")
	if _, ok := ix.Get("IR-2026-001"); ok {
		t.Error("анализ не удалён")
	}
}

func TestSearchTokensAndStatus(t *testing.T) {
	ix := New()
	ix.Put(a("IR-2026-001", "Этанол технический", "E1", "готов"))
	ix.Put(a("IR-2026-002", "Метанол", "M1", "новый"))
	ix.Put(a("IR-2026-003", "Этанол пищевой", "E2", "новый"))

	if got := ix.Search("этанол", ""); len(got) != 2 {
		t.Errorf("поиск 'этанол' вернул %d, ожидалось 2", len(got))
	}

	if got := ix.Search("этанол пищевой", ""); len(got) != 1 || got[0].ID != "IR-2026-003" {
		t.Errorf("поиск 'этанол пищевой' вернул %d записей", len(got))
	}

	if got := ix.Search("IR-2026-002", ""); len(got) != 1 || got[0].Product != "Метанол" {
		t.Errorf("поиск по ID не сработал")
	}

	if got := ix.Search("", "новый"); len(got) != 2 {
		t.Errorf("фильтр статуса вернул %d, ожидалось 2", len(got))
	}

	if got := ix.Search("этанол", "новый"); len(got) != 1 || got[0].ID != "IR-2026-003" {
		t.Errorf("комбинированный поиск вернул %d записей", len(got))
	}
}

func TestOrderNewestFirst(t *testing.T) {
	ix := New()
	ix.Put(a("IR-2026-002", "B", "", "новый"))
	ix.Put(a("IR-2026-001", "A", "", "новый"))
	ix.Put(a("IR-2026-1000", "C", "", "новый"))
	ids := []string{}
	for _, an := range ix.All() {
		ids = append(ids, an.ID)
	}
	want := []string{"IR-2026-1000", "IR-2026-002", "IR-2026-001"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("порядок[%d]=%s, ожидалось %s (полный: %v)", i, ids[i], want[i], ids)
		}
	}
}

func TestLoadReplaces(t *testing.T) {
	ix := New()
	ix.Put(a("IR-2026-001", "старый", "", "новый"))
	ix.Load([]*domain.Analysis{a("IR-2026-009", "новый", "", "готов")})
	if ix.Len() != 1 {
		t.Fatalf("Len=%d после Load, ожидалось 1", ix.Len())
	}
	if _, ok := ix.Get("IR-2026-001"); ok {
		t.Error("Load не заменил прежнее содержимое")
	}
}
