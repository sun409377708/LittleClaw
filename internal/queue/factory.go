package queue

import (
	"fmt"
	"time"

	"littleclaw/internal/config"
)

func NewStoreFromConfig(cfg *config.Config) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("queue config is nil")
	}
	backoffBase, err := time.ParseDuration(cfg.Queue.BackoffBase)
	if err != nil {
		return nil, fmt.Errorf("invalid queue.backoff_base: %w", err)
	}
	backoffMax, err := time.ParseDuration(cfg.Queue.BackoffMax)
	if err != nil {
		return nil, fmt.Errorf("invalid queue.backoff_max: %w", err)
	}
	return NewStoreWithOptions(Options{
		Dir:           cfg.Queue.Dir,
		DeadLetterDir: cfg.Queue.DeadLetterDir,
		BackoffBase:   backoffBase,
		BackoffMax:    backoffMax,
	}), nil
}
