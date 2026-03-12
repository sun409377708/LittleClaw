package queue

import "time"

type Options struct {
	Dir           string
	DeadLetterDir string
	BackoffBase   time.Duration
	BackoffMax    time.Duration
}
