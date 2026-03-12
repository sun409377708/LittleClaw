package queue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type WorkerLock struct {
	path string
}

func AcquireWorkerLock(path string) (*WorkerLock, error) {
	if path == "" {
		return nil, fmt.Errorf("worker lock path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create worker lock dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("worker lock is already held at %q", path)
		}
		return nil, fmt.Errorf("create worker lock file: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.Itoa(os.Getpid()) + "\n" + time.Now().UTC().Format(time.RFC3339) + "\n"); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("write worker lock file: %w", err)
	}
	return &WorkerLock{path: path}, nil
}

func (l *WorkerLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove worker lock: %w", err)
	}
	return nil
}
