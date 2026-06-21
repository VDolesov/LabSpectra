package service

import (
	"crypto/subtle"
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
	fs            *storage.FileStore
	reg           *storage.Registry
	ix            *index.Index
	lock          *storage.Lock
	logFile       *os.File
	adminPassword string
	products      []string
	mu            sync.Mutex
}

func adminPasswordFromEnv() (string, error) {
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		return v, nil
	}
	if os.Getenv("PORT") != "" {
		return "", fmt.Errorf("ADMIN_PASSWORD должен быть задан для публичного запуска")
	}
	return "123", nil
}

func (s *Service) CheckAdmin(pw string) bool {
	if pw == "" || s.adminPassword == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pw), []byte(s.adminPassword)) == 1
}

func New(root string) (*Service, error) {
	adminPassword, err := adminPasswordFromEnv()
	if err != nil {
		return nil, err
	}
	fs, err := storage.NewFileStore(root)
	if err != nil {
		return nil, err
	}
	products, err := storage.ReadProducts(fs.Paths.Products(), domain.Products())
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
		fs:            fs,
		reg:           storage.NewRegistry(fs.Paths),
		ix:            index.New(),
		lock:          lock,
		logFile:       logFile,
		adminPassword: adminPassword,
		products:      products,
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

func (s *Service) Products() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.products...)
}

func (s *Service) AddProduct(product string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	product = strings.TrimSpace(product)
	if product == "" {
		return nil, fmt.Errorf("продукт не должен быть пустым")
	}
	if s.validProductLocked(product) {
		return append([]string(nil), s.products...), nil
	}
	s.products = append(s.products, product)
	if err := storage.WriteProducts(s.fs.Paths.Products(), s.products); err != nil {
		return nil, err
	}
	slog.Info("добавлен продукт", "product", product)
	return append([]string(nil), s.products...), nil
}

func (s *Service) DeleteProduct(product string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	product = strings.TrimSpace(product)
	if product == "" {
		return nil, fmt.Errorf("продукт не указан")
	}
	for _, a := range s.ix.All() {
		if a.Product == product {
			return nil, fmt.Errorf("продукт %q используется в анализе %s", product, a.ID)
		}
	}
	idx := -1
	for i, p := range s.products {
		if p == product {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("продукт не найден: %q", product)
	}
	s.products = append(s.products[:idx], s.products[idx+1:]...)
	if err := storage.WriteProducts(s.fs.Paths.Products(), s.products); err != nil {
		return nil, err
	}
	slog.Info("удалён продукт", "product", product)
	return append([]string(nil), s.products...), nil
}

func (s *Service) validProductLocked(product string) bool {
	if product == "" {
		return true
	}
	for _, p := range s.products {
		if p == product {
			return true
		}
	}
	return false
}

type CreateInput struct {
	AnalysisDate  string `json:"analysis_date"`
	SynthesisDate string `json:"synthesis_date"`
	Product       string `json:"product"`
	Origin        string `json:"origin"`
	Source        string `json:"source"`
	Batch         string `json:"batch"`
	Operator      string `json:"operator"`
	SampleName    string `json:"sample_name"`
	Description   string `json:"description"`
	ShortResult   string `json:"short_result"`
	Status        string `json:"status"`
	Comment       string `json:"comment"`
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
	product := strings.TrimSpace(in.Product)
	if !s.validProductLocked(product) {
		return nil, fmt.Errorf("неизвестный продукт: %q", product)
	}
	source := strings.TrimSpace(in.Source)
	if !domain.ValidSource(source) {
		return nil, fmt.Errorf("неизвестное происхождение: %q", source)
	}

	a := &domain.Analysis{
		SchemaVersion: domain.SchemaVersion,
		ID:            id,
		AnalysisDate:  date,
		SynthesisDate: strings.TrimSpace(in.SynthesisDate),
		Product:       product,
		Origin:        strings.TrimSpace(in.Origin),
		Source:        source,
		Batch:         strings.TrimSpace(in.Batch),
		Operator:      strings.TrimSpace(in.Operator),
		SampleName:    strings.TrimSpace(in.SampleName),
		Description:   in.Description,
		ShortResult:   in.ShortResult,
		Status:        status,
		Comment:       in.Comment,
		Attachments:   domain.Attachments{Photos: []string{}, Spectra: []string{}},
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
	if !ok || a.Deleted {
		return nil, fmt.Errorf("анализ не найден: %s", id)
	}
	return a, nil
}

type Filter struct {
	Query         string
	Status        string
	AnalysisFrom  string
	AnalysisTo    string
	SynthesisFrom string
	SynthesisTo   string
}

func (s *Service) List(f Filter) []*domain.Analysis {
	res := s.ix.Search(f.Query, f.Status)
	out := make([]*domain.Analysis, 0, len(res))
	for _, a := range res {
		if a.Deleted {
			continue
		}
		if !inRange(a.AnalysisDate, f.AnalysisFrom, f.AnalysisTo) {
			continue
		}
		if !inRange(a.SynthesisDate, f.SynthesisFrom, f.SynthesisTo) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func (s *Service) ListDeleted() []*domain.Analysis {
	out := make([]*domain.Analysis, 0)
	for _, a := range s.ix.All() {
		if a.Deleted {
			out = append(out, a)
		}
	}
	return out
}

func (s *Service) setDeleted(id string, deleted bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.ix.Get(id)
	if !ok {
		return fmt.Errorf("анализ не найден: %s", id)
	}
	a := cur.Clone()
	a.Deleted = deleted
	a.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := s.fs.WriteCard(a); err != nil {
		return err
	}
	s.ix.Put(a)
	if err := s.rebuildRegistryLocked(); err != nil {
		return err
	}
	if deleted {
		slog.Info("анализ помечен на удаление", "id", id)
	} else {
		slog.Info("анализ восстановлен", "id", id)
	}
	return nil
}

func (s *Service) SoftDelete(id string) error { return s.setDeleted(id, true) }

func (s *Service) Restore(id string) error { return s.setDeleted(id, false) }

func inRange(d, from, to string) bool {
	if from == "" && to == "" {
		return true
	}
	if d == "" {
		return false
	}
	if from != "" && d < from {
		return false
	}
	if to != "" && d > to {
		return false
	}
	return true
}

type UpdateInput struct {
	AnalysisDate  string `json:"analysis_date"`
	SynthesisDate string `json:"synthesis_date"`
	Product       string `json:"product"`
	Origin        string `json:"origin"`
	Source        string `json:"source"`
	Batch         string `json:"batch"`
	Operator      string `json:"operator"`
	SampleName    string `json:"sample_name"`
	Description   string `json:"description"`
	ShortResult   string `json:"short_result"`
	Status        string `json:"status"`
	Comment       string `json:"comment"`
}

func (s *Service) Update(id string, in UpdateInput) (*domain.Analysis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.ix.Get(id)
	if !ok {
		return nil, fmt.Errorf("анализ не найден: %s", id)
	}
	if cur.Deleted {
		return nil, fmt.Errorf("анализ ожидает подтверждения удаления: %s", id)
	}
	product := strings.TrimSpace(in.Product)
	if !s.validProductLocked(product) {
		return nil, fmt.Errorf("неизвестный продукт: %q", product)
	}
	source := strings.TrimSpace(in.Source)
	if !domain.ValidSource(source) {
		return nil, fmt.Errorf("неизвестное происхождение: %q", source)
	}
	a := cur.Clone()
	a.AnalysisDate = strings.TrimSpace(in.AnalysisDate)
	a.SynthesisDate = strings.TrimSpace(in.SynthesisDate)
	a.Product = product
	a.Origin = strings.TrimSpace(in.Origin)
	a.Source = source
	a.Batch = strings.TrimSpace(in.Batch)
	a.Operator = strings.TrimSpace(in.Operator)
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
	if cur.Deleted {
		return nil, fmt.Errorf("анализ ожидает подтверждения удаления: %s", id)
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
	if cur.Deleted {
		return nil, fmt.Errorf("анализ ожидает подтверждения удаления: %s", id)
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

func (s *Service) Purge(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur, ok := s.ix.Get(id)
	if !ok {
		return fmt.Errorf("анализ не найден: %s", id)
	}
	if !cur.Deleted {
		return fmt.Errorf("сначала отправьте анализ на удаление: %s", id)
	}
	if err := s.fs.TrashSample(id); err != nil {
		return err
	}
	s.ix.Delete(id)
	slog.Info("анализ удалён окончательно", "id", id)
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
	a, ok := s.ix.Get(id)
	if !ok || a.Deleted {
		return "", fmt.Errorf("анализ не найден: %s", id)
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
	return len(activeOf(s.ix.All())), nil
}

func (s *Service) Backup() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return storage.Backup(s.fs.Paths)
}

func (s *Service) rebuildRegistryLocked() error {
	cards := activeOf(s.ix.All())
	sortByIDAsc(cards)
	return s.reg.Rebuild(cards)
}

func activeOf(cards []*domain.Analysis) []*domain.Analysis {
	out := make([]*domain.Analysis, 0, len(cards))
	for _, a := range cards {
		if !a.Deleted {
			out = append(out, a)
		}
	}
	return out
}

func (s *Service) reconcile() error {
	cards := s.ix.All()
	if !s.reg.Healthy() {
		if len(cards) == 0 {
			return nil
		}
		if _, err := s.reg.QuarantineCorrupted(); err != nil {
			return err
		}
		return s.rebuildAndCommit(cards)
	}
	regIDs, err := s.reg.IDs()
	if err != nil {
		return err
	}
	if !s.reg.SchemaOK() || !sameIDs(activeOf(cards), regIDs) {
		return s.rebuildAndCommit(cards)
	}
	return s.commitUncommitted(cards)
}

func (s *Service) rebuildAndCommit(cards []*domain.Analysis) error {
	if err := s.rebuildRegistryLocked(); err != nil {
		return err
	}
	return s.commitUncommitted(cards)
}

func (s *Service) commitUncommitted(cards []*domain.Analysis) error {
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
