package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ReadCharacteristics(path string, defaults []string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return uniqueNonEmpty(defaults), nil
		}
		return nil, err
	}
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		return uniqueNonEmpty(list), nil
	}
	var catalog map[string][]string
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	migrated := append([]string{}, defaults...)
	for _, list := range catalog {
		migrated = append(migrated, list...)
	}
	return uniqueNonEmpty(migrated), nil
}

func WriteCharacteristics(path string, list []string) error {
	list = uniqueNonEmpty(list)
	data, err := json.MarshalIndent(list, "", "  ")
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
