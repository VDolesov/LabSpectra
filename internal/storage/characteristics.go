package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ReadCharacteristics(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]string{}, nil
		}
		return nil, err
	}
	var catalog map[string][]string
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	return normalizeCatalog(catalog), nil
}

func WriteCharacteristics(path string, catalog map[string][]string) error {
	catalog = normalizeCatalog(catalog)
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".characteristics-*.json")
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
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func normalizeCatalog(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for product, list := range in {
		product = cleanValue(product)
		if product == "" {
			continue
		}
		list = uniqueNonEmpty(list)
		out[product] = list
	}
	return out
}
