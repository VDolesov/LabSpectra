package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"labspectra/internal/domain"
)

const cardFileName = "card.json"

func (fs *FileStore) CardPath(id string) string {
	return filepath.Join(fs.Paths.SampleDir(id), cardFileName)
}

func (fs *FileStore) WriteCard(a *domain.Analysis) error {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("сериализация card.json: %w", err)
	}
	dir := fs.Paths.SampleDir(a.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".card-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, fs.CardPath(a.ID))
}

func (fs *FileStore) ReadCard(id string) (*domain.Analysis, error) {
	data, err := os.ReadFile(fs.CardPath(id))
	if err != nil {
		return nil, err
	}
	var a domain.Analysis
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("разбор card.json (%s): %w", id, err)
	}
	return &a, nil
}

func (fs *FileStore) ReadAllCards() (cards []*domain.Analysis, broken map[string]error, err error) {
	ids, err := fs.ExistingIDs()
	if err != nil {
		return nil, nil, err
	}
	broken = map[string]error{}
	for _, id := range ids {
		a, e := fs.ReadCard(id)
		if e != nil {
			broken[id] = e
			continue
		}
		cards = append(cards, a)
	}
	return cards, broken, nil
}
