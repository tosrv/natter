package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type BlockedUser struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	BlockedAt   time.Time `json:"blocked_at"`
}

// Block records that blocker blocks blocked. Idempotent; ErrNotFound if the
// target does not exist. Self-block is rejected by the table's CHECK.
func (s *Store) Block(ctx context.Context, blocker, blocked uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		blocker, blocked)
	if pgCode(err) == "23503" {
		return ErrNotFound
	}
	return err
}

// Unblock removes the block row if present. Idempotent.
func (s *Store) Unblock(ctx context.Context, blocker, blocked uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`, blocker, blocked)
	return err
}

// ListBlocks lists the users blocker has blocked, newest first.
func (s *Store) ListBlocks(ctx context.Context, blocker uuid.UUID) ([]BlockedUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT u.id, coalesce(u.username,''), u.display_name, b.created_at
		 FROM blocks b JOIN users u ON u.id = b.blocked_id
		 WHERE b.blocker_id=$1 ORDER BY b.created_at DESC, u.id`, blocker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlockedUser{}
	for rows.Next() {
		var b BlockedUser
		if err := rows.Scan(&b.ID, &b.Username, &b.DisplayName, &b.BlockedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BlockRelated reports whether a blocks b or b blocks a.
func (s *Store) BlockRelated(ctx context.Context, a, b uuid.UUID) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM blocks WHERE (blocker_id=$1 AND blocked_id=$2) OR (blocker_id=$2 AND blocked_id=$1))`,
		a, b).Scan(&ok)
	return ok, err
}
