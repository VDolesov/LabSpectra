package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func ReadProducts(path string, defaults []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return uniqueNonEmpty(defaults), nil
		}
		return nil, err
	}
	var products []string
	if err := json.Unmarshal(data, &products); err != nil {
		return nil, err
	}
	products = uniqueNonEmpty(products)
	if len(products) == 0 {
		return uniqueNonEmpty(defaults), nil
	}
	return products, nil
}

func WriteProducts(path string, products []string) error {
	products = uniqueNonEmpty(products)
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".products-*.json")
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

func uniqueNonEmpty(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
