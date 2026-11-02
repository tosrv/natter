package store

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// SetUsername sets the user's username (already normalized by the caller).
// The first choice (NULL username) is free; the same name is a no-op; any
// other change increments username_changes (CHECK limits it to 3).
func (s *Store) SetUsername(ctx context.Context, userID uuid.UUID, username string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx,
		`UPDATE users SET username=$2,
		   username_changes = username_changes + CASE WHEN username IS NULL THEN 0 ELSE 1 END
		 WHERE id=$1 AND username IS DISTINCT FROM $2 RETURNING `+userCols,
		userID, username))
	switch {
	case err == nil:
		return u, nil
	case pgCode(err) == "23505":
		return User{}, ErrUsernameTaken
	case pgCode(err) == "23514":
		return User{}, ErrUsernameChangeLimit
	case errors.Is(err, ErrNotFound):
		// Either no such user or the name is unchanged (no-op).
		return s.GetUser(ctx, userID)
	}
	return User{}, err
}

type UserBrief struct {
	ID       uuid.UUID
	Username string
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchUsers finds Google users whose username starts with prefix
// (case-insensitive, wildcards literal), excluding excludeID.
func (s *Store) SearchUsers(ctx context.Context, prefix string, excludeID uuid.UUID, limit int) ([]UserBrief, error) {
	pat := likeEscaper.Replace(strings.ToLower(prefix)) + "%"
	rows, err := s.pool.Query(ctx,
		`SELECT id, username FROM users
		 WHERE kind='google' AND username IS NOT NULL AND id <> $2
		   AND lower(username) LIKE $1 ESCAPE '\'
		 ORDER BY username LIMIT $3`, pat, excludeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserBrief{}
	for rows.Next() {
		var b UserBrief
		if err := rows.Scan(&b.ID, &b.Username); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
