package session

import (
	"fmt"
	"sync"
	"time"
)

// ProgressRecorder updates the persisted session as pull commands advance.
type ProgressRecorder struct {
	session *PullSession
	mu      sync.Mutex
}

// NewProgressRecorder returns a recorder bound to a pull session.
func NewProgressRecorder(ps *PullSession) *ProgressRecorder {
	return &ProgressRecorder{session: ps}
}

// Start records the initial endpoint metadata before fetching pages.
func (r *ProgressRecorder) Start(endpoint string, metadata map[string]string, initialPage, initialCount int) error {
	return r.record(endpoint, metadata, initialPage, initialCount)
}

// Page updates the persisted state with the latest page progress.
func (r *ProgressRecorder) Page(endpoint string, metadata map[string]string, page int, count int) error {
	return r.record(endpoint, metadata, page, count)
}

func (r *ProgressRecorder) record(endpoint string, metadata map[string]string, page int, count int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.session.Endpoint = endpoint
	r.session.LastPage = page
	r.session.FetchedCount = count
	r.session.Metadata = cloneMetadata(metadata)
	r.session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := SavePull(r.session); err != nil {
		return fmt.Errorf("failed to persist pull session: %w", err)
	}
	return nil
}

func cloneMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
