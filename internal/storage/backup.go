package storage

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"time"
)

func Backup(p Paths) (string, error) {
	if err := os.MkdirAll(p.Backups(), 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(p.Backups(), "labspectra-"+time.Now().Format("20060102-150405")+".zip")
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()

	if _, err := os.Stat(p.Registry()); err == nil {
		if err := addToZip(zw, p.Registry(), "registry.xlsx"); err != nil {
			return "", err
		}
	}

	if _, err := os.Stat(p.Samples()); err == nil {
		err = filepath.Walk(p.Samples(), func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(p.Root, path)
			if err != nil {
				return err
			}
			return addToZip(zw, path, filepath.ToSlash(rel))
		})
		if err != nil {
			return "", err
		}
	}
	return dst, nil
}

func addToZip(zw *zip.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}
