package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExpiredGuests returns guests whose latest conversation activity (or
// created_at when no message was ever sent) is older than olderThan.
func (s *Store) ExpiredGuests(ctx context.Context, olderThan time.Time) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id FROM users u
		LEFT JOIN conversations c ON c.user_a = u.id OR c.user_b = u.id
		WHERE u.kind = 'guest'
		GROUP BY u.id, u.created_at
		HAVING COALESCE(max(c.last_activity_at), u.created_at) < $1`, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteInactiveConversations removes conversations (and their messages via
// CASCADE) whose last activity is older than olderThan.
func (s *Store) DeleteInactiveConversations(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM conversations WHERE last_activity_at < $1`, olderThan)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
