package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"littleclaw/internal/types"
)

func persistRun(run *types.RunResult) error {
	if run == nil {
		return fmt.Errorf("run is nil")
	}

	if err := os.MkdirAll("runs", 0o755); err != nil {
		return fmt.Errorf("create runs dir: %w", err)
	}

	payload, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}

	path := filepath.Join("runs", run.RunID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write run file: %w", err)
	}

	return nil
}

func LoadRun(path string) (*types.RunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read run file: %w", err)
	}

	var run types.RunResult
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("parse run file: %w", err)
	}

	return &run, nil
}

func ListRuns(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read runs dir: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}
