package store

// Semaphore bounds concurrent operations within a single warm function
// instance — used to cap simultaneous upstream Drive requests so a burst
// of seeks/parallel catalog fetches can't open unbounded connections.
// This is intentionally process-local (not Redis-backed): it's a soft,
// best-effort throttle per instance, not a correctness requirement, so it
// doesn't need cross-instance coordination.
type Semaphore struct {
	ch chan struct{}
}

func NewSemaphore(n int) *Semaphore {
	return &Semaphore{ch: make(chan struct{}, n)}
}

func (s *Semaphore) Acquire() { s.ch <- struct{}{} }
func (s *Semaphore) Release() { <-s.ch }
