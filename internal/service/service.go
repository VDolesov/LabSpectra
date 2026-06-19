package service

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"labspectra/internal/domain"
	"labspectra/internal/index"
	"labspectra/internal/storage"
)

type Service struct {
	fs      *storage.FileStore
	reg     *storage.Registry
	ix      *index.Index
	lock    *storage.Lock
	logFile *os.File
	mu      sync.Mutex
}

func New(root string) (*Service, error) {
	fs, err := storage.NewFileStore(root)
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(fs.Paths.Logs(), "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo})))

	lock, err := storage.AcquireLock(fs.Paths.Lock())
	if err != nil {
		logFile.Close()
		return nil, err
	}
	s := &Service{
		fs:      fs,
		reg:     storage.NewRegistry(fs.Paths),
		ix:      index.New(),
		lock:    lock,
		logFile: logFile,
	}

	cards, broken, err := fs.ReadAllCards()
	if err != nil {
		lock.Release()
		logFile.Close()
		return nil, err
	}
	for id, e := range broken {
		slog.Warn("повреждённая карточка пропущена", "id", id, "err", e)
	}
	s.ix.Load(cards)

	if err := s.reconcile(); err != nil {
		lock.Release()
		logFile.Close()
		return nil, err
	}
	slog.Info("запуск", "data", fs.Paths.Root, "count", len(cards))
	return s, nil
}

func (s *Service) Close() error {
	err := s.lock.Release()
	if s.logFile != nil {
		s.logFile.Close()
	}
	return err
}

func (s *Service) Root() string { return s.fs.Paths.Root }

type CreateInput struct {
	AnalysisDate string `json:"analysis_date"`
	Product      string `json:"product"`
	Batch        string `json:"batch"`
	SampleName   string `json:"sample_name"`
	Description  string `json:"description"`
	ShortResult  string `json:"short_result"`
	Status       string `json:"status"`
	Comment      string `json:"comment"`
}

func (s *Service) Create(in CreateInput) (*domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids, err := s.fs.ExistingIDs()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	id := domain.NextID(now.Year(), ids)

	date := strings.TrimSpace(in.AnalysisDate)
	if date == "" {
		date = now.Format("2006-01-02")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = string(domain.StatusNew)
	}

	a := &domain.Analysis{
		SchemaVersion: domain.SchemaVersion,
		ID:            id,
		AnalysisDate:  date,
		Product:       strings.TrimSpace(in.Product),
		Batch:         strings.TrimSpace(in.Batch),
		SampleName:    strings.TrimSpace(in.SampleName),
		Description:   in.Description,
		ShortResult:   in.ShortResult,
		Status:        status,
		Comment:       in.Comment,
		Attachments:   domain.Attachments{Photos: []string{}, Spectra: []string{}, Reports: []string{}},
		CreatedAt:     now.Format(time.RFC3339),
		UpdatedAt:     now.Format(time.RFC3339),
		Committed:     false,
	}

	if err := s.fs.CreateSampleDirs(id); err != nil {
		return nil, err
	}
	if err := s.fs.WriteCard(a); err != nil {
		return nil, err
	}
	if err := s.reg.Upsert(a); err != nil {
		return nil, fmt.Errorf("анализ создан, но реестр не обновлён: %w", err)
	}
	a.Committed = true
	if err := s.fs.WriteCard(a); err != nil {
		return nil, err
	}
	s.ix.Put(a)
	slog.Info("создан анализ", "id", a.ID)
	return a, nil
}

func (s *Service) Get(id string) (*domain.Analysis, error) {
	a, ok := s.ix.Get(id)
	if !ok {
		return nil, fmt.Errorf("анализ не найден: %s", id)
	}
	return a, nil
}

func (s *Service) List(query, status string) []*domain.Analysis {
	return s.ix.Search(query, status)
}

type UpdateInput struct {
	AnalysisDate string `json:"analysis_date"`
	Product      string `json:"product"`
	Batch        string `json:"batch"`
	SampleName   string `json:"sample_name"`
	Description  string `json:"description"`
	ShortResult  string `json:"short_result"`
	Status       string `json:"status"`
	Comment      string `json:"comment"`
}

func (s *Service) Update(id string, in UpdateInput) (*domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.ix.Get(id)
	if !ok {
		return nil, fmt.Errorf("анализ не найден: %s", id)
	}
	a := cur.Clone()
	a.AnalysisDate = strings.TrimSpace(in.AnalysisDate)
	a.Product = strings.TrimSpace(in.Product)
	a.Batch = strings.TrimSpace(in.Batch)
	a.SampleName = strings.TrimSpace(in.SampleName)
	a.Description = in.Description
	a.ShortResult = in.ShortResult
	a.Status = strings.TrimSpace(in.Status)
	a.Comment = in.Comment
	a.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := s.persist(a); err != nil {
		return nil, err
	}
	slog.Info("изменён анализ", "id", a.ID)
	return a, nil
}

func (s *Service) AddAttachment(id string, kind domain.Kind, origName string, src io.Reader) (*domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.ix.Get(id)
	if !ok {
		return nil, fmt.Errorf("анализ не найден: %s", id)
	}
	rel, err := s.fs.SaveAttachment(id, kind, origName, src)
	if err != nil {
		return nil, err
	}
	a := cur.Clone()
	list := a.Attachments.For(kind)
	*list = append(*list, rel)
	a.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := s.persist(a); err != nil {
		return nil, err
	}
	slog.Info("добавлено вложение", "id", id, "kind", string(kind), "file", rel)
	return a, nil
}

func (s *Service) RemoveAttachment(id string, kind domain.Kind, rel string) (*domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.ix.Get(id)
	if !ok {
		return nil, fmt.Errorf("анализ не найден: %s", id)
	}
	a := cur.Clone()
	list := a.Attachments.For(kind)
	if list == nil {
		return nil, fmt.Errorf("неизвестный тип вложения: %q", kind)
	}
	idx := indexOf(*list, rel)
	if idx == -1 {
		return nil, fmt.Errorf("вложение не найдено: %s", rel)
	}
	if err := s.fs.TrashAttachment(id, rel); err != nil {
		return nil, err
	}
	*list = append((*list)[:idx], (*list)[idx+1:]...)
	a.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := s.persist(a); err != nil {
		return nil, err
	}
	slog.Info("удалено вложение", "id", id, "kind", string(kind), "file", rel)
	return a, nil
}

func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.ix.Get(id); !ok {
		return fmt.Errorf("анализ не найден: %s", id)
	}
	if err := s.fs.TrashSample(id); err != nil {
		return err
	}
	s.ix.Delete(id)
	slog.Info("удалён анализ", "id", id)
	return s.rebuildRegistryLocked()
}

func (s *Service) persist(a *domain.Analysis) error {
	if err := s.fs.WriteCard(a); err != nil {
		return err
	}
	if err := s.reg.Upsert(a); err != nil {
		return err
	}
	s.ix.Put(a)
	return nil
}

func (s *Service) AttachmentFile(id, rel string) (string, error) {
	if !domain.ValidID(id) {
		return "", fmt.Errorf("некорректный ID")
	}
	if rel == "" || strings.ContainsRune(rel, ':') {
		return "", fmt.Errorf("недопустимый путь")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("недопустимый путь")
	}
	base := s.fs.Paths.SampleDir(id)
	abs := filepath.Join(base, clean)
	rp, err := filepath.Rel(base, abs)
	if err != nil || rp == ".." || strings.HasPrefix(rp, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("путь вне папки анализа")
	}
	return abs, nil
}

func (s *Service) OpenFolder(id string) error {
	if !domain.ValidID(id) {
		return fmt.Errorf("некорректный ID: %q", id)
	}
	return openInOS(s.fs.Paths.SampleDir(id))
}

func (s *Service) OpenRegistry() error {
	p := s.fs.Paths.Registry()
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("registry.xlsx ещё не создан — создайте первый анализ")
	}
	return openInOS(p)
}

func (s *Service) RebuildRegistry() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rebuildRegistryLocked(); err != nil {
		return 0, err
	}
	return s.ix.Len(), nil
}

func (s *Service) Backup() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return storage.Backup(s.fs.Paths)
}

func (s *Service) rebuildRegistryLocked() error {
	cards := s.ix.All()
	sortByIDAsc(cards)
	return s.reg.Rebuild(cards)
}

func (s *Service) reconcile() error {
	cards := s.ix.All()
	if len(cards) == 0 {
		return nil
	}
	needRebuild := !s.reg.Healthy()
	if needRebuild {
		if _, err := s.reg.QuarantineCorrupted(); err != nil {
			return err
		}
	} else {
		regIDs, err := s.reg.IDs()
		if err != nil {
			return err
		}
		needRebuild = !sameIDs(cards, regIDs)
	}
	if needRebuild {
		if err := s.rebuildRegistryLocked(); err != nil {
			return err
		}
	}
	for _, a := range cards {
		if a.Committed {
			continue
		}
		c := a.Clone()
		c.Committed = true
		if err := s.fs.WriteCard(c); err != nil {
			return err
		}
		s.ix.Put(c)
	}
	return nil
}

func sameIDs(cards []*domain.Analysis, regIDs []string) bool {
	if len(cards) != len(regIDs) {
		return false
	}
	set := make(map[string]bool, len(cards))
	for _, a := range cards {
		set[a.ID] = true
	}
	for _, id := range regIDs {
		if !set[id] {
			return false
		}
	}
	return true
}

func indexOf(list []string, v string) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return -1
}

func sortByIDAsc(cards []*domain.Analysis) {
	sort.Slice(cards, func(i, j int) bool { return idLess(cards[i].ID, cards[j].ID) })
}

func idLess(a, b string) bool {
	ya, sa, oka := domain.ParseID(a)
	yb, sb, okb := domain.ParseID(b)
	if oka && okb {
		if ya != yb {
			return ya < yb
		}
		return sa < sb
	}
	return a < b
}

func openInOS(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
