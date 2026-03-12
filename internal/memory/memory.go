package memory

import (
	"sync"

	"littleclaw/internal/types"
)

type Store struct {
	mu    sync.Mutex
	steps []types.Step
}

func New() *Store {
	return &Store{
		steps: make([]types.Step, 0, 8),
	}
}

func (s *Store) Append(step types.Step) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.steps = append(s.steps, step)
}

func (s *Store) Steps() []types.Step {
	s.mu.Lock()
	defer s.mu.Unlock()

	steps := make([]types.Step, len(s.steps))
	copy(steps, s.steps)
	return steps
}
