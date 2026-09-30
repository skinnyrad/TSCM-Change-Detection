package state

import (
	"sync"
	"time"

	"github.com/skinnyrad/tscm-change-detection/internal/imgproc"
)

// MaxBatchImages caps a batch so memory stays bounded (each image costs its
// display JPEG plus ~0.3–0.6 MB of analysis data).
const MaxBatchImages = 1000

// BatchItem is one uploaded batch image. Only a display-resolution JPEG is
// kept; analysis decodes it on demand.
type BatchItem struct {
	ID      string
	Name    string
	W, H    int // original dimensions
	Display []byte
}

// BatchJob reports the progress of a batch analysis.
type BatchJob struct {
	State    string    `json:"state"` // idle | running | done | error
	Done     int       `json:"done"`
	Total    int       `json:"total"`
	Error    string    `json:"error,omitempty"`
	Started  time.Time `json:"started"`
	Duration float64   `json:"duration_s"`
}

// BatchStore holds the batch (many images of one scene) and its analysis.
// Like Store it is process-global and single-user.
type BatchStore struct {
	mu     sync.RWMutex
	items  []*BatchItem
	job    BatchJob
	result *imgproc.BatchResult
	ids    []string // item IDs in result order
	cache  map[string][]byte
}

// Batch is the process-wide batch store.
var Batch = &BatchStore{job: BatchJob{State: "idle"}}

// Add appends items unless an analysis is running; returns the new count.
func (b *BatchStore) Add(items ...*BatchItem) (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.job.State == "running" || len(b.items)+len(items) > MaxBatchImages {
		return len(b.items), false
	}
	b.items = append(b.items, items...)
	b.result, b.ids, b.cache = nil, nil, nil
	b.job = BatchJob{State: "idle"}
	return len(b.items), true
}

// Clear removes everything unless an analysis is running.
func (b *BatchStore) Clear() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.job.State == "running" {
		return false
	}
	b.items, b.result, b.ids, b.cache = nil, nil, nil, nil
	b.job = BatchJob{State: "idle"}
	return true
}

// Items returns a snapshot of the items.
func (b *BatchStore) Items() []*BatchItem {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]*BatchItem(nil), b.items...)
}

// Item finds an item by ID.
func (b *BatchStore) Item(id string) *BatchItem {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, it := range b.items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

// Job returns the current job status.
func (b *BatchStore) Job() BatchJob {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.job
}

// Start marks a job as running and returns the items to analyse, or false if
// one is already running or there is nothing to do.
func (b *BatchStore) Start() ([]*BatchItem, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.job.State == "running" || len(b.items) < 3 {
		return nil, false
	}
	b.job = BatchJob{State: "running", Total: len(b.items), Started: time.Now()}
	b.result, b.ids, b.cache = nil, nil, nil
	return append([]*BatchItem(nil), b.items...), true
}

// Progress updates the running job's progress.
func (b *BatchStore) Progress(done int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.job.Done = done
}

// Finish stores the result (or error) of the running job.
func (b *BatchStore) Finish(res *imgproc.BatchResult, ids []string, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.job.Duration = time.Since(b.job.Started).Seconds()
	if err != nil {
		b.job.State, b.job.Error = "error", err.Error()
		return
	}
	b.job.State, b.job.Done = "done", b.job.Total
	b.result, b.ids, b.cache = res, ids, map[string][]byte{}
}

// Result returns the analysis result and the item ID for each result index.
func (b *BatchStore) Result() (*imgproc.BatchResult, []string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.result, b.ids
}

// Cached returns a rendered artefact, computing and caching it on first use.
func (b *BatchStore) Cached(key string, render func() ([]byte, error)) ([]byte, error) {
	b.mu.RLock()
	v, ok := b.cache[key]
	b.mu.RUnlock()
	if ok {
		return v, nil
	}
	v, err := render()
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.cache != nil {
		b.cache[key] = v
	}
	b.mu.Unlock()
	return v, nil
}
