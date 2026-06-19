package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"labspectra/internal/domain"
)

const sheetName = "Реестр"

const linkSep = "; "

const snapshotKeep = 10

var registryHeaders = []string{
	"ID анализа",
	"дата анализа",
	"продукт",
	"партия",
	"название образца",
	"описание",
	"краткий результат анализа",
	"статус",
	"путь к папке анализа",
	"ссылки на фотографии",
	"ссылки на спектры",
	"ссылки на отчёты",
	"комментарий",
	"дата создания",
	"дата изменения",
}

type Registry struct {
	paths Paths
	file  *excelize.File
	mtime time.Time
}

func NewRegistry(paths Paths) *Registry {
	return &Registry{paths: paths}
}

func (r *Registry) ensureFile() (*excelize.File, error) {
	if r.file != nil {
		if fi, err := os.Stat(r.paths.Registry()); err == nil && fi.ModTime().Equal(r.mtime) {
			return r.file, nil
		}
		r.file.Close()
		r.file = nil
	}
	if _, err := os.Stat(r.paths.Registry()); err == nil {
		f, err := excelize.OpenFile(r.paths.Registry())
		if err != nil {
			return nil, err
		}
		r.file = f
		r.recordMtime()
		return f, nil
	}
	f, err := r.newFile()
	if err != nil {
		return nil, err
	}
	r.file = f
	return f, nil
}

func (r *Registry) recordMtime() {
	if fi, err := os.Stat(r.paths.Registry()); err == nil {
		r.mtime = fi.ModTime()
	}
}

func (r *Registry) newFile() (*excelize.File, error) {
	f := excelize.NewFile()
	idx, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")
	if err := r.writeHeader(f); err != nil {
		return nil, err
	}
	return f, nil
}

func (r *Registry) writeHeader(f *excelize.File) error {
	row := make([]interface{}, len(registryHeaders))
	for i, h := range registryHeaders {
		row[i] = h
	}
	if err := f.SetSheetRow(sheetName, "A1", &row); err != nil {
		return err
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err == nil {
		lastCol, _ := excelize.ColumnNumberToName(len(registryHeaders))
		f.SetCellStyle(sheetName, "A1", lastCol+"1", style)
	}

	widths := []float64{14, 12, 18, 12, 20, 28, 24, 10, 24, 28, 28, 24, 24, 20, 20}
	for i, w := range widths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, col, col, w)
	}
	return nil
}

func (r *Registry) rowValues(a *domain.Analysis) []interface{} {
	rel := func(list []string) string {
		out := make([]string, len(list))
		for i, p := range list {
			out[i] = r.paths.RelAttachment(a.ID, p)
		}
		return strings.Join(out, linkSep)
	}
	return []interface{}{
		a.ID,
		a.AnalysisDate,
		a.Product,
		a.Batch,
		a.SampleName,
		a.Description,
		a.ShortResult,
		a.Status,
		r.paths.RelSampleDir(a.ID),
		rel(a.Attachments.Photos),
		rel(a.Attachments.Spectra),
		rel(a.Attachments.Reports),
		a.Comment,
		a.CreatedAt,
		a.UpdatedAt,
	}
}

func findRow(f *excelize.File, id string) (int, error) {
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, err
	}
	for i, row := range rows {
		if len(row) > 0 && row[0] == id {
			return i + 1, nil
		}
	}
	return 0, nil
}

func (r *Registry) Upsert(a *domain.Analysis) error {
	f, err := r.ensureFile()
	if err != nil {
		return fmt.Errorf("открытие реестра: %w", err)
	}
	rowNum, err := findRow(f, a.ID)
	if err != nil {
		return err
	}
	if rowNum == 0 {
		rows, _ := f.GetRows(sheetName)
		rowNum = len(rows) + 1
	}
	cell, _ := excelize.CoordinatesToCellName(1, rowNum)
	vals := r.rowValues(a)
	if err := f.SetSheetRow(sheetName, cell, &vals); err != nil {
		return err
	}
	return r.save()
}

func (r *Registry) Rebuild(cards []*domain.Analysis) error {
	if err := r.snapshot(); err != nil {
		return err
	}
	f, err := r.newFile()
	if err != nil {
		return err
	}
	for i, a := range cards {
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		vals := r.rowValues(a)
		if err := f.SetSheetRow(sheetName, cell, &vals); err != nil {
			return err
		}
	}
	if r.file != nil {
		r.file.Close()
	}
	r.file = f
	return r.save()
}

func (r *Registry) save() error {
	if r.file == nil {
		return nil
	}
	dir := filepath.Dir(r.paths.Registry())
	tmp := filepath.Join(dir, fmt.Sprintf(".registry-%d.xlsx", time.Now().UnixNano()))
	if err := r.file.SaveAs(tmp); err != nil {
		return fmt.Errorf("сохранение реестра: %w", err)
	}
	if err := os.Rename(tmp, r.paths.Registry()); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("не удалось обновить registry.xlsx (возможно, файл открыт в Excel — закройте его): %w", err)
	}
	r.recordMtime()
	return nil
}

func (r *Registry) snapshot() error {
	src := r.paths.Registry()
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.paths.Backups(), 0o755); err != nil {
		return err
	}
	dst := filepath.Join(r.paths.Backups(), "registry-"+time.Now().Format("20060102-150405")+".xlsx")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	return pruneSnapshots(r.paths.Backups(), snapshotKeep)
}

func pruneSnapshots(dir string, keep int) error {
	matches, err := filepath.Glob(filepath.Join(dir, "registry-*.xlsx"))
	if err != nil {
		return err
	}
	if len(matches) <= keep {
		return nil
	}
	sort.Strings(matches)
	for _, p := range matches[:len(matches)-keep] {
		os.Remove(p)
	}
	return nil
}

func (r *Registry) IDs() ([]string, error) {
	if _, err := os.Stat(r.paths.Registry()); err != nil {
		return nil, nil
	}
	f, err := excelize.OpenFile(r.paths.Registry())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	var ids []string
	for i, row := range rows {
		if i == 0 {
			continue
		}
		if len(row) > 0 && row[0] != "" {
			ids = append(ids, row[0])
		}
	}
	return ids, nil
}

func (r *Registry) Healthy() bool {
	if _, err := os.Stat(r.paths.Registry()); err != nil {
		return false
	}
	f, err := excelize.OpenFile(r.paths.Registry())
	if err != nil {
		return false
	}
	defer f.Close()
	for _, s := range f.GetSheetList() {
		if s == sheetName {
			return true
		}
	}
	return false
}

func (r *Registry) QuarantineCorrupted() (string, error) {
	src := r.paths.Registry()
	if _, err := os.Stat(src); err != nil {
		return "", nil
	}
	dst := filepath.Join(filepath.Dir(src),
		"registry.corrupted-"+time.Now().Format("20060102-150405")+".xlsx")
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}
