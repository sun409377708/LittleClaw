package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ServeLock struct {
	path string
}

func AcquireServeLock(path string) (*ServeLock, error) {
	if path == "" {
		return nil, fmt.Errorf("lock path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("scheduler lock is already held at %q", path)
		}
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	defer file.Close()

	payload := []byte(strconv.Itoa(os.Getpid()) + "\n" + time.Now().UTC().Format(time.RFC3339) + "\n")
	if _, err := file.Write(payload); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write lock file: %w", err)
	}

	return &ServeLock{path: path}, nil
}

func (l *ServeLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove lock file: %w", err)
	}
	return nil
}
