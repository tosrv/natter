package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// CreateSession issues a session token (32 random bytes, base64url). Only its
// SHA-256 is stored.
func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, ttl time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`,
		hashToken(token), userID, time.Now().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

// UserBySession returns the session's user, or ErrNotFound if the token is
// unknown or expired.
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT u.id, u.kind, u.google_sub, u.email, u.username, u.username_changes, u.is_developer, u.created_at
		 FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`, hashToken(token)))
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hashToken(token))
	return err
}
