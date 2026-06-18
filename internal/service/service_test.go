package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"labspectra/internal/domain"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { svc.Close() })
	return svc
}

func TestCreateProducesFoldersCardAndRegistry(t *testing.T) {
	svc := newTestService(t)

	a, err := svc.Create(CreateInput{Product: "Продукт А", Batch: "П-01", SampleName: "Фильтрат"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID != "IR-"+yearStr()+"-001" {
		t.Errorf("первый ID = %q", a.ID)
	}
	if !a.Committed {
		t.Error("анализ должен быть committed после создания")
	}

	root := svc.Root()

	for _, sub := range []string{"photos", "spectra", "reports"} {
		dir := filepath.Join(root, "samples", a.ID, sub)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			t.Errorf("нет подпапки %s", dir)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "samples", a.ID, "card.json")); err != nil {
		t.Errorf("нет card.json: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "registry.xlsx")); err != nil {
		t.Errorf("нет registry.xlsx: %v", err)
	}
}

func TestSequentialIDs(t *testing.T) {
	svc := newTestService(t)
	ids := []string{}
	for i := 0; i < 3; i++ {
		a, err := svc.Create(CreateInput{Product: "X"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, a.ID)
	}
	want := []string{"IR-" + yearStr() + "-001", "IR-" + yearStr() + "-002", "IR-" + yearStr() + "-003"}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ID[%d]=%q, want %q", i, ids[i], want[i])
		}
	}
}

func TestAddAttachmentAndSpectraSharedNumber(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.Create(CreateInput{Product: "X"})

	a, err := svc.AddAttachment(a.ID, domain.KindSpectrum, "data.pdf", strings.NewReader("pdf"))
	if err != nil {
		t.Fatal(err)
	}

	a, err = svc.AddAttachment(a.ID, domain.KindSpectrum, "data.txt", strings.NewReader("txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Attachments.Spectra) != 2 {
		t.Fatalf("ожидалось 2 спектра, получено %d", len(a.Attachments.Spectra))
	}
	if a.Attachments.Spectra[0] != "spectra/spectrum_1.pdf" || a.Attachments.Spectra[1] != "spectra/spectrum_1.txt" {
		t.Errorf("неверные имена спектров: %v", a.Attachments.Spectra)
	}
}

func TestSearchAndFilter(t *testing.T) {
	svc := newTestService(t)
	svc.Create(CreateInput{Product: "Этанол", Batch: "E1", Status: string(domain.StatusDone)})
	svc.Create(CreateInput{Product: "Метанол", Batch: "M1", Status: string(domain.StatusNew)})

	got := svc.List("этанол", "")
	if len(got) != 1 || got[0].Product != "Этанол" {
		t.Errorf("поиск 'этанол' вернул %d записей", len(got))
	}
	got = svc.List("", string(domain.StatusNew))
	if len(got) != 1 || got[0].Product != "Метанол" {
		t.Errorf("фильтр по статусу вернул %d записей", len(got))
	}
}

func TestRecoverFromCorruptedRegistry(t *testing.T) {
	root := t.TempDir()
	svc, _ := New(root)
	svc.Create(CreateInput{Product: "A"})
	svc.Create(CreateInput{Product: "B"})

	regPath := filepath.Join(root, "registry.xlsx")
	if err := os.WriteFile(regPath, []byte("это не xlsx"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc.Close()

	svc2, err := New(root)
	if err != nil {
		t.Fatalf("New на повреждённом реестре: %v", err)
	}
	defer svc2.Close()
	if !svc2.reg.Healthy() {
		t.Fatal("реестр не восстановлен")
	}

	matches, _ := filepath.Glob(filepath.Join(root, "registry.corrupted-*.xlsx"))
	if len(matches) == 0 {
		t.Error("повреждённый файл не помещён в карантин")
	}

	list := svc2.List("", "")
	if len(list) != 2 {
		t.Errorf("после восстановления %d анализов, ожидалось 2", len(list))
	}
}

func TestRebuildRegistry(t *testing.T) {
	svc := newTestService(t)
	svc.Create(CreateInput{Product: "A"})
	svc.Create(CreateInput{Product: "B"})
	n, err := svc.RebuildRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("пересобрано %d строк, ожидалось 2", n)
	}
}

func TestDeleteMovesToTrashAndUpdatesIndex(t *testing.T) {
	root := t.TempDir()
	svc, _ := New(root)
	defer svc.Close()
	a1, _ := svc.Create(CreateInput{Product: "A"})
	svc.Create(CreateInput{Product: "B"})

	if err := svc.Delete(a1.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(svc.List("", "")) != 1 {
		t.Errorf("после удаления осталось %d анализов, ожидался 1", len(svc.List("", "")))
	}
	if _, ok := svc.ix.Get(a1.ID); ok {
		t.Error("удалённый анализ остался в индексе")
	}

	if _, err := os.Stat(filepath.Join(root, "samples", a1.ID)); !os.IsNotExist(err) {
		t.Error("папка анализа не удалена из samples/")
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".trash", a1.ID+"-*"))
	if len(matches) == 0 {
		t.Error("папка анализа не перенесена в .trash/")
	}
}

func TestUpsertCreatesNoSnapshots(t *testing.T) {
	root := t.TempDir()
	svc, _ := New(root)
	defer svc.Close()
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(CreateInput{Product: "X"}); err != nil {
			t.Fatal(err)
		}
	}
	snaps, _ := filepath.Glob(filepath.Join(root, "backups", "registry-*.xlsx"))
	if len(snaps) != 0 {
		t.Errorf("Upsert создал %d снимков реестра, ожидалось 0", len(snaps))
	}
}

func TestBackupCreatesZip(t *testing.T) {
	svc := newTestService(t)
	svc.Create(CreateInput{Product: "A"})
	path, err := svc.Backup()
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("архив не создан: %v", err)
	}
	if st.Size() == 0 {
		t.Error("архив пуст")
	}
	if filepath.Ext(path) != ".zip" {
		t.Errorf("ожидался .zip, получено %s", path)
	}
}

func TestAttachmentFileContainment(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.Create(CreateInput{Product: "X"})

	p, err := svc.AttachmentFile(a.ID, "photos/photo_1.jpg")
	if err != nil {
		t.Fatalf("валидный путь отклонён: %v", err)
	}
	if !strings.Contains(p, filepath.Join("samples", a.ID, "photos")) {
		t.Errorf("неожиданный путь: %s", p)
	}

	for _, bad := range []string{"..", "../../secret", "photos/../../../etc", "C:/Windows/win.ini"} {
		if _, err := svc.AttachmentFile(a.ID, bad); err == nil {
			t.Errorf("обход каталога не отклонён: %q", bad)
		}
	}
	if _, err := svc.AttachmentFile("не-id", "photos/x"); err == nil {
		t.Error("некорректный ID не отклонён")
	}
}

func TestReconcileRebuildsOnOrphanRow(t *testing.T) {
	root := t.TempDir()
	svc, _ := New(root)
	svc.Create(CreateInput{Product: "A"})
	b, _ := svc.Create(CreateInput{Product: "B"})

	if err := os.RemoveAll(filepath.Join(root, "samples", b.ID)); err != nil {
		t.Fatal(err)
	}
	svc.Close()

	svc2, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	if len(svc2.List("", "")) != 1 {
		t.Errorf("в индексе %d анализов, ожидался 1", len(svc2.List("", "")))
	}
	ids, err := svc2.reg.IDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "IR-"+yearStr()+"-001" {
		t.Errorf("реестр не пересобран после сироты: %v", ids)
	}
}

func yearStr() string {
	return strconv.Itoa(time.Now().Year())
}
