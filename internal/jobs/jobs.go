// Package jobs runs periodic cleanup: expired guests and conversation retention.
package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store"
)

const (
	GuestTTL      = 30 * time.Minute
	RetentionHour = 3
)

type Disconnector interface{ Disconnect(uuid.UUID) }

// CleanupGuests deletes guests inactive for GuestTTL and disconnects them.
func CleanupGuests(ctx context.Context, st *store.Store, hub Disconnector, now func() time.Time) (int, error) {
	ids, err := st.ExpiredGuests(ctx, now().Add(-GuestTTL))
	if err != nil {
		return 0, err
	}
	n := 0
	var firstErr error
	for _, id := range ids {
		if _, _, err := st.EndGuest(ctx, id); err != nil { // TODO(Task 2): use outcome
			if errors.Is(err, store.ErrNotFound) {
				continue // already ended (e.g. concurrent logout)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		hub.Disconnect(id)
		n++
	}
	return n, firstErr
}

// RunRetention deletes conversations inactive for more than one month.
func RunRetention(ctx context.Context, st *store.Store, now func() time.Time) (int64, error) {
	return st.DeleteInactiveConversations(ctx, now().AddDate(0, -1, 0))
}

// NextDailyRun returns the next occurrence of hour:00 strictly after now.
func NextDailyRun(now time.Time, hour int) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !t.After(now) {
		t = time.Date(now.Year(), now.Month(), now.Day()+1, hour, 0, 0, 0, now.Location())
	}
	return t
}

// Run blocks until ctx is done.
func Run(ctx context.Context, st *store.Store, hub Disconnector, limiters []*ratelimit.Limiter, now func() time.Time) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	daily := time.NewTimer(time.Until(NextDailyRun(now(), RetentionHour)))
	defer daily.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			n, err := CleanupGuests(ctx, st, hub, now)
			if err != nil {
				slog.Error("guest cleanup", "err", err)
			}
			if n > 0 {
				slog.Info("guests deleted", "count", n)
			}
			for _, l := range limiters {
				l.Sweep()
			}
		case <-daily.C:
			n, err := RunRetention(ctx, st, now)
			if err != nil {
				slog.Error("retention", "err", err)
			} else {
				slog.Info("conversations deleted", "count", n)
			}
			daily.Reset(time.Until(NextDailyRun(now(), RetentionHour)))
		}
	}
}
