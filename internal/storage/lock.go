package storage

import (
	"fmt"
	"os"
)

type Lock struct {
	f *os.File
}

func AcquireLock(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("папка данных уже используется другим экземпляром LabSpectra")
	}
	return &Lock{f: f}, nil
}

func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	unlockFile(l.f)
	err := l.f.Close()
	l.f = nil
	return err
}
