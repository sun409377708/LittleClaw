package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"littleclaw/internal/types"
)

type Entry struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	SourceID   string          `json:"source_id"`
	ParentID   string          `json:"parent_id,omitempty"`
	Subject    string          `json:"subject"`
	Summary    string          `json:"summary"`
	Status     types.RunStatus `json:"status"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
	Tags       []string        `json:"tags,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
}

type Match struct {
	Entry Entry `json:"entry"`
	Score int   `json:"score"`
}

type SearchOptions struct {
	Query     string
	Limit     int
	Kind      string
	Status    string
	Tags      []string
	Since     *time.Time
	Until     *time.Time
	FieldExpr map[string]string
}

func PersistEntry(dir string, entry Entry) error {
	if strings.TrimSpace(dir) == "" {
		dir = "memories"
	}
	if entry.ID == "" {
		entry.ID = types.NewID("memory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	payload, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory entry: %w", err)
	}
	path := filepath.Join(dir, entry.ID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write memory entry: %w", err)
	}
	return nil
}

func LoadEntry(path string) (*Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read memory entry: %w", err)
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parse memory entry: %w", err)
	}
	return &entry, nil
}

func ListEntries(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
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

func Search(dir, query string, limit int) ([]Match, error) {
	return SearchWithOptions(dir, SearchOptions{
		Query: query,
		Limit: limit,
	})
}

func SearchWithOptions(dir string, opts SearchOptions) ([]Match, error) {
	paths, err := ListEntries(dir)
	if err != nil {
		return nil, err
	}
	if opts.Limit <= 0 {
		opts.Limit = 5
	}

	queryTokens := tokenize(opts.Query)
	if len(queryTokens) == 0 {
		queryTokens = []string{"*"}
	}

	matches := make([]Match, 0, len(paths))
	for _, path := range paths {
		entry, err := LoadEntry(path)
		if err != nil {
			return nil, err
		}
		if !entryMatchesFilters(*entry, opts) {
			continue
		}
		score := scoreEntry(*entry, queryTokens)
		if score == 0 && queryTokens[0] != "*" {
			continue
		}
		matches = append(matches, Match{
			Entry: *entry,
			Score: score,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			return matches[i].Entry.FinishedAt.After(matches[j].Entry.FinishedAt)
		}
		return matches[i].Score > matches[j].Score
	})

	if opts.Limit > len(matches) {
		opts.Limit = len(matches)
	}
	return matches[:opts.Limit], nil
}

func tokenize(raw string) []string {
	replacer := strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		",", " ",
		".", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		`"`, " ",
		"'", " ",
	)
	normalized := strings.ToLower(replacer.Replace(raw))
	fields := strings.Fields(normalized)
	seen := make(map[string]struct{}, len(fields))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		tokens = append(tokens, field)
	}
	return tokens
}

func scoreEntry(entry Entry, queryTokens []string) int {
	if len(queryTokens) == 1 && queryTokens[0] == "*" {
		return 1
	}
	corpus := strings.ToLower(entry.Subject + "\n" + entry.Summary + "\n" + strings.Join(entry.Tags, " "))
	score := 0
	for _, token := range queryTokens {
		if strings.Contains(corpus, token) {
			score++
		}
	}
	return score
}

func TruncateText(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if limit <= 0 {
		limit = 400
	}
	runes := []rune(raw)
	if len(runes) <= limit {
		return raw
	}
	return string(runes[:limit]) + "..."
}

func entryMatchesFilters(entry Entry, opts SearchOptions) bool {
	if opts.Kind != "" && !strings.EqualFold(entry.Kind, opts.Kind) {
		return false
	}
	if opts.Status != "" && !strings.EqualFold(string(entry.Status), opts.Status) {
		return false
	}
	if len(opts.Tags) > 0 && !hasAllTags(entry.Tags, opts.Tags) {
		return false
	}
	if opts.Since != nil && entry.FinishedAt.Before(*opts.Since) {
		return false
	}
	if opts.Until != nil && entry.FinishedAt.After(*opts.Until) {
		return false
	}
	if len(opts.FieldExpr) > 0 && !matchesFieldFilters(entry, opts.FieldExpr) {
		return false
	}
	return true
}

func hasAllTags(entryTags, required []string) bool {
	if len(required) == 0 {
		return true
	}
	available := make(map[string]struct{}, len(entryTags))
	for _, tag := range entryTags {
		available[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	for _, tag := range required {
		if _, ok := available[strings.ToLower(strings.TrimSpace(tag))]; !ok {
			return false
		}
	}
	return true
}

func matchesFieldFilters(entry Entry, filters map[string]string) bool {
	for key, value := range filters {
		if !matchesField(entry, key, value) {
			return false
		}
	}
	return true
}

func matchesField(entry Entry, key, value string) bool {
	switch key {
	case "id":
		return entry.ID == value
	case "kind":
		return strings.EqualFold(entry.Kind, value)
	case "source_id":
		return entry.SourceID == value
	case "parent_id":
		return entry.ParentID == value
	case "subject":
		return strings.Contains(strings.ToLower(entry.Subject), strings.ToLower(value))
	case "summary":
		return strings.Contains(strings.ToLower(entry.Summary), strings.ToLower(value))
	case "status":
		return strings.EqualFold(string(entry.Status), value)
	case "tag", "tags":
		return hasAllTags(entry.Tags, []string{value})
	default:
		if strings.HasPrefix(key, "metadata.") {
			return metadataFieldMatches(entry.Metadata, strings.TrimPrefix(key, "metadata."), value)
		}
		return false
	}
}

func metadataFieldMatches(metadata map[string]any, key, value string) bool {
	if metadata == nil {
		return false
	}
	raw, ok := metadata[key]
	if !ok {
		return false
	}
	switch typed := raw.(type) {
	case string:
		return strings.EqualFold(typed, value) || strings.Contains(strings.ToLower(typed), strings.ToLower(value))
	case float64:
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return typed == parsed
		}
		return false
	case int:
		if parsed, err := strconv.Atoi(value); err == nil {
			return typed == parsed
		}
		return false
	case bool:
		return strings.EqualFold(strconv.FormatBool(typed), value)
	default:
		return fmt.Sprint(raw) == value
	}
}
