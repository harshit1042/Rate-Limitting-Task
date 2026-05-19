package ratelimit

import (
	"sort"
	"sync"
	"time"
)

// Limiter enforces a rolling per-user request window in memory.
type Limiter struct {
	maxPerWindow int
	window       time.Duration
	now          func() time.Time

	mu    sync.Mutex
	users map[string]*userState
}

type userState struct {
	lock       sync.Mutex
	timestamps []time.Time
	accepted   int64
	rejected   int64
}

// NewLimiter creates an in-memory sliding-window limiter.
func NewLimiter(maxPerWindow int, window time.Duration) *Limiter {
	return &Limiter{
		maxPerWindow: maxPerWindow,
		window:       window,
		now:          time.Now,
		users:        make(map[string]*userState),
	}
}

// TryAccept records a request when capacity remains.
func (l *Limiter) TryAccept(userID string) (requestsInWindow int, allowed bool) {
	st := l.stateFor(userID)
	st.lock.Lock()
	defer st.lock.Unlock()

	now := l.now()
	l.prune(st, now)
	if len(st.timestamps) >= l.maxPerWindow {
		st.rejected++
		return len(st.timestamps), false
	}
	st.timestamps = append(st.timestamps, now)
	st.accepted++
	return len(st.timestamps), true
}

// Snapshot returns all user stats sorted by user_id.
func (l *Limiter) Snapshot() []UserStats {
	l.mu.Lock()
	ids := make([]string, 0, len(l.users))
	for id := range l.users {
		ids = append(ids, id)
	}
	l.mu.Unlock()

	out := make([]UserStats, 0, len(ids))
	now := l.now()
	for _, id := range ids {
		st := l.stateFor(id)
		st.lock.Lock()
		l.prune(st, now)
		out = append(out, UserStats{
			UserID:                  id,
			TotalAccepted:           st.accepted,
			TotalRejected:           st.rejected,
			RequestsInCurrentWindow: len(st.timestamps),
		})
		st.lock.Unlock()
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UserID < out[j].UserID
	})
	return out
}

// WindowDuration exposes the configured window (for Retry-After).
func (l *Limiter) WindowDuration() time.Duration {
	return l.window
}

func (l *Limiter) stateFor(userID string) *userState {
	l.mu.Lock()
	defer l.mu.Unlock()
	st, ok := l.users[userID]
	if !ok {
		st = &userState{}
		l.users[userID] = st
	}
	return st
}

func (l *Limiter) prune(st *userState, now time.Time) {
	cutoff := now.Add(-l.window)
	i := 0
	for i < len(st.timestamps) && st.timestamps[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		st.timestamps = st.timestamps[i:]
	}
}
