package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"littleclaw/internal/memory"
	"littleclaw/internal/types"
)

const runsDir = "schedule_runs"

func PersistRun(run *RunResult) error {
	return PersistRunWithMemoryDir(run, "memories")
}

func PersistRunWithMemoryDir(run *RunResult, memoryDir string) error {
	if run == nil {
		return fmt.Errorf("schedule run is nil")
	}
	if run.RunID == "" {
		run.RunID = types.NewID("schedule")
	}
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return fmt.Errorf("create schedule runs dir: %w", err)
	}
	payload, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedule run: %w", err)
	}
	path := filepath.Join(runsDir, run.RunID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write schedule run file: %w", err)
	}
	if err := memory.PersistEntry(memoryDir, run.MemoryEntry()); err != nil {
		return fmt.Errorf("persist schedule memory: %w", err)
	}
	return nil
}

func LoadRun(path string) (*RunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schedule run file: %w", err)
	}
	var run RunResult
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("parse schedule run file: %w", err)
	}
	return &run, nil
}

func ListRuns() ([]string, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(runsDir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}
