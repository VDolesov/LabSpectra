package storage

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"labspectra/internal/domain"
)

type FileStore struct {
	Paths Paths
}

func NewFileStore(root string) (*FileStore, error) {
	fs := &FileStore{Paths: NewPaths(root)}
	for _, dir := range []string{fs.Paths.Root, fs.Paths.Samples(), fs.Paths.Backups(), fs.Paths.Logs()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("создание папки %s: %w", dir, err)
		}
	}
	return fs, nil
}

func (fs *FileStore) CreateSampleDirs(id string) error {
	for _, kind := range []domain.Kind{domain.KindPhoto, domain.KindSpectrum, domain.KindReport} {
		dir := fs.Paths.KindDir(id, kind.Folder())
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("создание подпапки %s: %w", dir, err)
		}
	}
	return nil
}

func (fs *FileStore) ExistingIDs() ([]string, error) {
	entries, err := os.ReadDir(fs.Paths.Samples())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && domain.ValidID(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func (fs *FileStore) SaveAttachment(id string, kind domain.Kind, origName string, src io.Reader) (string, error) {
	if !kind.Valid() {
		return "", fmt.Errorf("неизвестный тип вложения: %q", kind)
	}
	ext := strings.ToLower(filepath.Ext(origName))
	dir := fs.Paths.KindDir(id, kind.Folder())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	seq := fs.nextSeq(dir, kind, ext)
	finalName := fmt.Sprintf("%s_%d%s", kind.Prefix(), seq, ext)
	finalPath := filepath.Join(dir, finalName)

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		os.Remove(tmpName)
		return "", err
	}
	return path.Join(kind.Folder(), finalName), nil
}

func (fs *FileStore) nextSeq(dir string, kind domain.Kind, ext string) int {
	entries, _ := os.ReadDir(dir)
	prefix := kind.Prefix() + "_"

	if kind == domain.KindSpectrum {
		max := 0
		hasExt := map[int]bool{}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasPrefix(n, prefix) {
				continue
			}
			base := strings.TrimSuffix(n, filepath.Ext(n))
			numStr := strings.TrimPrefix(base, prefix)
			var num int
			if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
				continue
			}
			if num > max {
				max = num
			}
			if strings.EqualFold(filepath.Ext(n), ext) {
				hasExt[num] = true
			}
		}

		if max > 0 && !hasExt[max] {
			return max
		}
		return max + 1
	}

	max := 0
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, prefix) {
			continue
		}
		base := strings.TrimSuffix(n, filepath.Ext(n))
		numStr := strings.TrimPrefix(base, prefix)
		var num int
		if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
			continue
		}
		if num > max {
			max = num
		}
	}
	return max + 1
}

func (fs *FileStore) TrashSample(id string) error {
	if err := os.MkdirAll(fs.Paths.Trash(), 0o755); err != nil {
		return err
	}
	src := fs.Paths.SampleDir(id)
	dst := filepath.Join(fs.Paths.Trash(), id+"-"+time.Now().Format("20060102-150405"))
	return os.Rename(src, dst)
}

func (fs *FileStore) TrashAttachment(id, rel string) error {
	srcAbs := fs.Paths.AbsFromAnalysisRel(id, rel)
	stamp := time.Now().Format("20060102-150405")
	dstDir := filepath.Join(fs.Paths.Trash(), id, filepath.Dir(filepath.FromSlash(rel)))
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(dstDir, stamp+"-"+filepath.Base(rel))
	return os.Rename(srcAbs, dst)
}
