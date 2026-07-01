package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	
	"sort"
	"sync"
	"time"

	"github.com/0embsd/devrec/internal/collectors"
)

// Snapshot is the complete output of a single snapshot run.
type Snapshot struct {
	SessionID   string              `json:"session_id"`
	Label       string              `json:"label"`
	CollectedAt time.Time           `json:"collected_at"`
	Results     []collectors.Result `json:"results"`
	Error       string              `json:"error,omitempty"`
}

// Engine runs a set of collectors concurrently and produces a Snapshot.
type Engine struct {
	registry collectors.Registry
	timeout  time.Duration
}

// NewEngine creates a snapshot engine.
func NewEngine(reg collectors.Registry, timeout time.Duration) *Engine {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Engine{registry: reg, timeout: timeout}
}

// Run executes all registered collectors in parallel.
func (e *Engine) Run(ctx context.Context, sessionID string, label string, names []string, customs []*collectors.CustomCollector) *Snapshot {
	snap := &Snapshot{
		SessionID:   sessionID,
		Label:       label,
		CollectedAt: time.Now().UTC(),
	}

	// Gather collectors to run.
	var todo []collectors.Collector
	for _, name := range names {
		if c, ok := e.registry[name]; ok {
			todo = append(todo, c)
		}
	}
	for _, c := range customs {
		todo = append(todo, c)
	}

	if len(todo) == 0 {
		return snap
	}

	results := make([]collectors.Result, len(todo))
	var wg sync.WaitGroup
	wg.Add(len(todo))

	for i, c := range todo {
		go func(idx int, col collectors.Collector) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results[idx] = collectors.Result{
						Collector: col.Name(),
						Success:   false,
						Error:     fmt.Sprintf("panic: %v", r),
						Duration:  "0s",
					}
				}
			}()

			colCtx, cancel := context.WithTimeout(ctx, e.timeout)
			defer cancel()

			t0 := time.Now()
			data, err := col.Collect(colCtx)
			d := time.Since(t0)

			if err != nil {
				results[idx] = collectors.Result{
					Collector: col.Name(),
					Success:   false,
					Error:     err.Error(),
					Duration:  d.String(),
				}
			} else {
				results[idx] = collectors.Result{
					Collector: col.Name(),
					Success:   true,
					Data:      data,
					Duration:  d.String(),
				}
			}
		}(i, c)
	}

	wg.Wait()

	// Sort by collector name for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Collector < results[j].Collector
	})
	snap.Results = results
	return snap
}

// WriteJSON writes a snapshot to a JSON file atomically.
func WriteJSON(snap *Snapshot, path string) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return os.Rename(tmp, path)
}

// ComputeDiff compares pre and post snapshots, producing a structured diff.
func ComputeDiff(pre, post *Snapshot) interface{} {
	type diffEntry struct {
		Collector string      `json:"collector"`
		Field     string      `json:"field,omitempty"`
		Pre       interface{} `json:"pre"`
		Post      interface{} `json:"post"`
	}

	var entries []diffEntry
	preMap := make(map[string]collectors.Result, len(pre.Results))
	for _, r := range pre.Results {
		preMap[r.Collector] = r
	}
	postMap := make(map[string]collectors.Result, len(post.Results))
	for _, r := range post.Results {
		postMap[r.Collector] = r
	}

	// Check all post collectors against pre.
	for name, postR := range postMap {
		preR, ok := preMap[name]
		if !ok {
			entries = append(entries, diffEntry{
				Collector: name,
				Field:     "(new)",
				Post:      postR.Data,
			})
			continue
		}

		// Compare JSON representation.
		preJSON, _ := json.Marshal(preR.Data)
		postJSON, _ := json.Marshal(postR.Data)
		if string(preJSON) != string(postJSON) {
			entries = append(entries, diffEntry{
				Collector: name,
				Pre:       preR.Data,
				Post:      postR.Data,
			})
		}
		delete(preMap, name)
	}

	// Remaining pre collectors not in post.
	for name, preR := range preMap {
		entries = append(entries, diffEntry{
			Collector: name,
			Field:     "(removed)",
			Pre:       preR.Data,
		})
	}

	return entries
}


// WriteJSONRaw writes arbitrary data as JSON to a file atomically.
func WriteJSONRaw(data interface{}, path string) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return os.Rename(tmp, path)
}
