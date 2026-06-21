package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

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

	a, err := svc.Create(CreateInput{Product: "R2531", Batch: "П-01", SampleName: "Фильтрат"})
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

	for _, sub := range []string{"photos", "spectra"} {
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
		a, err := svc.Create(CreateInput{Product: "R2531"})
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
	a, _ := svc.Create(CreateInput{Product: "R2531"})

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
	svc.Create(CreateInput{Product: "R2531", Batch: "E1", Status: string(domain.StatusDone)})
	svc.Create(CreateInput{Product: "V00S9", Batch: "M1", Status: string(domain.StatusNew)})

	got := svc.List(Filter{Query: "r2531"})
	if len(got) != 1 || got[0].Product != "R2531" {
		t.Errorf("поиск 'r2531' вернул %d записей", len(got))
	}
	got = svc.List(Filter{Status: string(domain.StatusNew)})
	if len(got) != 1 || got[0].Product != "V00S9" {
		t.Errorf("фильтр по статусу вернул %d записей", len(got))
	}
}

func TestRecoverFromCorruptedRegistry(t *testing.T) {
	root := t.TempDir()
	svc, _ := New(root)
	svc.Create(CreateInput{Product: "R2531"})
	svc.Create(CreateInput{Product: "V00S9"})

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

	list := svc2.List(Filter{})
	if len(list) != 2 {
		t.Errorf("после восстановления %d анализов, ожидалось 2", len(list))
	}
}

func TestRebuildRegistry(t *testing.T) {
	svc := newTestService(t)
	svc.Create(CreateInput{Product: "R2531"})
	svc.Create(CreateInput{Product: "V00S9"})
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
	a1, _ := svc.Create(CreateInput{Product: "R2531"})
	svc.Create(CreateInput{Product: "V00S9"})

	if err := svc.SoftDelete(a1.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if err := svc.Purge(a1.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if len(svc.List(Filter{})) != 1 {
		t.Errorf("после удаления осталось %d анализов, ожидался 1", len(svc.List(Filter{})))
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
		if _, err := svc.Create(CreateInput{Product: "R2531"}); err != nil {
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
	svc.Create(CreateInput{Product: "R2531"})
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
	a, _ := svc.Create(CreateInput{Product: "R2531"})

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
	svc.Create(CreateInput{Product: "R2531"})
	b, _ := svc.Create(CreateInput{Product: "V00S9"})

	if err := os.RemoveAll(filepath.Join(root, "samples", b.ID)); err != nil {
		t.Fatal(err)
	}
	svc.Close()

	svc2, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	if len(svc2.List(Filter{})) != 1 {
		t.Errorf("в индексе %d анализов, ожидался 1", len(svc2.List(Filter{})))
	}
	ids, err := svc2.reg.IDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "IR-"+yearStr()+"-001" {
		t.Errorf("реестр не пересобран после сироты: %v", ids)
	}
}

func TestProductValidation(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Create(CreateInput{Product: "НЕТ-ТАКОГО"}); err == nil {
		t.Error("несуществующий продукт принят")
	}
	if _, err := svc.Create(CreateInput{Product: "R2531"}); err != nil {
		t.Errorf("валидный продукт отклонён: %v", err)
	}
	if _, err := svc.Create(CreateInput{Product: ""}); err != nil {
		t.Errorf("пустой продукт отклонён: %v", err)
	}
}

func TestProductManagement(t *testing.T) {
	root := t.TempDir()
	svc, err := New(root)
	if err != nil {
		t.Fatal(err)
	}

	products, err := svc.AddProduct("NEW-01")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddProduct("TEMP-REMOVE"); err != nil {
		t.Fatal(err)
	}
	if products, err = svc.DeleteProduct("TEMP-REMOVE"); err != nil {
		t.Fatalf("удаление неиспользуемого продукта: %v", err)
	} else if contains(products, "TEMP-REMOVE") {
		t.Fatalf("удалённый продукт остался в списке: %v", products)
	}
	if !contains(products, "NEW-01") {
		t.Fatalf("новый продукт не добавлен: %v", products)
	}
	if _, err := svc.Create(CreateInput{Product: "NEW-01"}); err != nil {
		t.Fatalf("анализ с новым продуктом не создан: %v", err)
	}
	if _, err := svc.DeleteProduct("NEW-01"); err == nil {
		t.Error("удаление используемого продукта разрешено")
	}
	svc.Close()

	svc2, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	if !contains(svc2.Products(), "NEW-01") {
		t.Errorf("продукт не сохранился после перезапуска: %v", svc2.Products())
	}
}

func TestDateFilter(t *testing.T) {
	svc := newTestService(t)
	svc.Create(CreateInput{Product: "R2531", AnalysisDate: "2026-01-10", SynthesisDate: "2026-01-01"})
	svc.Create(CreateInput{Product: "V00S9", AnalysisDate: "2026-02-20", SynthesisDate: "2026-02-01"})
	svc.Create(CreateInput{Product: "PR4832", AnalysisDate: "2026-03-30", SynthesisDate: "2026-03-01"})

	got := svc.List(Filter{AnalysisFrom: "2026-02-01", AnalysisTo: "2026-02-28"})
	if len(got) != 1 || got[0].AnalysisDate != "2026-02-20" {
		t.Errorf("фильтр по дате анализа вернул %d записей", len(got))
	}
	got = svc.List(Filter{SynthesisFrom: "2026-03-01"})
	if len(got) != 1 || got[0].Product != "PR4832" {
		t.Errorf("фильтр по дате синтеза вернул %d записей", len(got))
	}
	got = svc.List(Filter{AnalysisFrom: "2026-01-01", AnalysisTo: "2026-12-31"})
	if len(got) != 3 {
		t.Errorf("широкий диапазон вернул %d, ожидалось 3", len(got))
	}
}

func TestReconcileRebuildsOnSchemaChange(t *testing.T) {
	root := t.TempDir()
	svc, _ := New(root)
	svc.Create(CreateInput{Product: "R2531"})
	svc.Close()

	f, err := excelize.OpenFile(filepath.Join(root, "registry.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	f.SetCellValue("Реестр", "A1", "устаревший заголовок")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	svc2, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	if !svc2.reg.SchemaOK() {
		t.Error("схема реестра не восстановлена при старте")
	}
	if len(svc2.List(Filter{})) != 1 {
		t.Error("данные потеряны при пересборке схемы")
	}
}

func TestSoftDeleteAndRestore(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.Create(CreateInput{Product: "R2531"})
	svc.Create(CreateInput{Product: "V00S9"})

	if err := svc.SoftDelete(a.ID); err != nil {
		t.Fatal(err)
	}
	if len(svc.List(Filter{})) != 1 {
		t.Errorf("после мягкого удаления в списке %d, ожидался 1", len(svc.List(Filter{})))
	}
	if len(svc.ListDeleted()) != 1 || svc.ListDeleted()[0].ID != a.ID {
		t.Errorf("удалённый не попал в список недавно удалённых")
	}
	ids, _ := svc.reg.IDs()
	if len(ids) != 1 {
		t.Errorf("в реестре %d строк, ожидалась 1 (удалённый исключён)", len(ids))
	}
	if n, err := svc.RebuildRegistry(); err != nil || n != 1 {
		t.Errorf("пересборка вернула (%d, %v), ожидалось 1 активная строка", n, err)
	}

	if err := svc.Restore(a.ID); err != nil {
		t.Fatal(err)
	}
	if len(svc.List(Filter{})) != 2 {
		t.Errorf("после восстановления в списке %d, ожидалось 2", len(svc.List(Filter{})))
	}
	if len(svc.ListDeleted()) != 0 {
		t.Errorf("после восстановления список удалённых не пуст")
	}
}

func TestDeletedAnalysisIsNotPubliclyAccessibleOrMutable(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.Create(CreateInput{Product: "R2531"})
	if _, err := svc.AddAttachment(a.ID, domain.KindPhoto, "photo.jpg", strings.NewReader("jpg")); err != nil {
		t.Fatal(err)
	}
	if err := svc.SoftDelete(a.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Get(a.ID); err == nil {
		t.Error("Get вернул мягко удалённый анализ")
	}
	if _, err := svc.Update(a.ID, UpdateInput{Product: "R2531"}); err == nil {
		t.Error("Update разрешил править мягко удалённый анализ")
	}
	if _, err := svc.AddAttachment(a.ID, domain.KindPhoto, "photo.jpg", strings.NewReader("jpg")); err == nil {
		t.Error("AddAttachment разрешил добавить файл в мягко удалённый анализ")
	}
	if _, err := svc.RemoveAttachment(a.ID, domain.KindPhoto, "photos/photo_1.jpg"); err == nil {
		t.Error("RemoveAttachment разрешил удалить файл из мягко удалённого анализа")
	}
	if _, err := svc.AttachmentFile(a.ID, "photos/photo_1.jpg"); err == nil {
		t.Error("AttachmentFile отдал файл мягко удалённого анализа")
	}
}

func TestCheckAdmin(t *testing.T) {
	svc := newTestService(t)
	if !svc.CheckAdmin("123") {
		t.Error("верный пароль отклонён")
	}
	if svc.CheckAdmin("000") || svc.CheckAdmin("") {
		t.Error("неверный/пустой пароль принят")
	}
}

func TestAdminPasswordRequiredForPublicRun(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("ADMIN_PASSWORD", "")
	if _, err := New(t.TempDir()); err == nil {
		t.Error("публичный запуск без ADMIN_PASSWORD разрешён")
	}
}

func yearStr() string {
	return strconv.Itoa(time.Now().Year())
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
