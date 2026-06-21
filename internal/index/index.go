package index

import (
	"sort"
	"strings"
	"sync"

	"labspectra/internal/domain"
)

type entry struct {
	a   *domain.Analysis
	key string
}

type Index struct {
	mu    sync.RWMutex
	byID  map[string]*entry
	order []string
}

func New() *Index {
	return &Index{byID: make(map[string]*entry)}
}

func searchKey(a *domain.Analysis) string {
	return strings.ToLower(strings.Join([]string{
		a.ID, a.Product, a.Origin, a.Source, a.Batch, a.SampleName,
		a.Operator, a.ShortResult, a.Description, a.Comment, a.Status, a.AnalysisDate, a.SynthesisDate,
	}, "\n"))
}

func (ix *Index) Load(cards []*domain.Analysis) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.byID = make(map[string]*entry, len(cards))
	for _, a := range cards {
		ix.byID[a.ID] = &entry{a: a, key: searchKey(a)}
	}
	ix.resortLocked()
}

func (ix *Index) Put(a *domain.Analysis) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	_, existed := ix.byID[a.ID]
	ix.byID[a.ID] = &entry{a: a, key: searchKey(a)}
	if !existed {
		ix.order = append(ix.order, a.ID)
		ix.resortLocked()
	}
}

func (ix *Index) Delete(id string) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if _, ok := ix.byID[id]; !ok {
		return
	}
	delete(ix.byID, id)
	for i, x := range ix.order {
		if x == id {
			ix.order = append(ix.order[:i], ix.order[i+1:]...)
			break
		}
	}
}

func (ix *Index) Get(id string) (*domain.Analysis, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	e, ok := ix.byID[id]
	if !ok {
		return nil, false
	}
	return e.a, true
}

func (ix *Index) Len() int {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return len(ix.byID)
}

func (ix *Index) Search(query, status string) []*domain.Analysis {
	tokens := strings.Fields(strings.ToLower(query))
	status = strings.TrimSpace(status)
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	out := make([]*domain.Analysis, 0, len(ix.order))
	for _, id := range ix.order {
		e := ix.byID[id]
		if status != "" && e.a.Status != status {
			continue
		}
		if !matchAll(e.key, tokens) {
			continue
		}
		out = append(out, e.a)
	}
	return out
}

func (ix *Index) All() []*domain.Analysis {
	return ix.Search("", "")
}

func matchAll(key string, tokens []string) bool {
	for _, t := range tokens {
		if !strings.Contains(key, t) {
			return false
		}
	}
	return true
}

func (ix *Index) resortLocked() {
	if cap(ix.order) < len(ix.byID) {
		ix.order = make([]string, 0, len(ix.byID))
	} else {
		ix.order = ix.order[:0]
	}
	for id := range ix.byID {
		ix.order = append(ix.order, id)
	}
	sort.Slice(ix.order, func(i, j int) bool { return idGreater(ix.order[i], ix.order[j]) })
}

func idGreater(a, b string) bool {
	ya, sa, oka := domain.ParseID(a)
	yb, sb, okb := domain.ParseID(b)
	if oka && okb {
		if ya != yb {
			return ya > yb
		}
		return sa > sb
	}
	return a > b
}
