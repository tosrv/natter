package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type User struct {
	ID              uuid.UUID
	Kind            string
	GoogleSub       *string
	Email           *string
	Username        *string
	DisplayName     string
	UsernameChanges int
	IsDeveloper     bool
	CreatedAt       time.Time
	DeactivatedAt   *time.Time
}

const userCols = `id, kind, google_sub, email, username, display_name, username_changes, is_developer, created_at, deactivated_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Kind, &u.GoogleSub, &u.Email, &u.Username, &u.DisplayName,
		&u.UsernameChanges, &u.IsDeveloper, &u.CreatedAt, &u.DeactivatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// takeSlot increments the used-slot counter; the row lock serializes
// concurrent callers and the CHECK constraint rejects the 301st.
func takeSlot(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `UPDATE user_slots SET used = used + 1 WHERE id = 1`)
	if pgCode(err) == "23514" {
		return ErrSlotsFull
	}
	return err
}

// UpsertGoogleUser logs in an existing Google user (no slot used) or creates
// a new one, taking a slot.
func (s *Store) UpsertGoogleUser(ctx context.Context, sub, email, displayName string, isDeveloper bool) (User, bool, error) {
	if displayName == "" {
		displayName = "User"
	}
	var (
		u       User
		created bool
	)
	// Two attempts: a concurrent insert of the same sub makes the second
	// attempt take the update path.
	for attempt := 0; attempt < 2; attempt++ {
		errRetry := errors.New("retry")
		err := s.WithTx(ctx, func(tx pgx.Tx) error {
			var err error
			u, err = scanUser(tx.QueryRow(ctx,
				`UPDATE users SET email=$2, is_developer=$3 WHERE google_sub=$1 RETURNING `+userCols,
				sub, email, isDeveloper))
			if err == nil {
				created = false
				return nil
			}
			if !errors.Is(err, ErrNotFound) {
				return err
			}
			if err := takeSlot(ctx, tx); err != nil {
				return err
			}
			u, err = scanUser(tx.QueryRow(ctx,
				`INSERT INTO users (kind, google_sub, email, display_name, is_developer) VALUES ('google',$1,$2,$4,$3)
				 ON CONFLICT (google_sub) DO NOTHING RETURNING `+userCols,
				sub, email, isDeveloper, displayName))
			if errors.Is(err, ErrNotFound) {
				return errRetry // lost a race; roll back the slot and retry
			}
			created = err == nil
			return err
		})
		if errors.Is(err, errRetry) {
			continue
		}
		return u, created, err
	}
	return User{}, false, errors.New("upsert google user: too much contention")
}

// createConversationTx returns the conversation between a and b, creating it
// (ON CONFLICT DO NOTHING, then SELECT) if needed. It inserts a conversation between a and b plus both
// membership rows, normalizing the user_a < user_b ordering.
func createConversationTx(ctx context.Context, tx pgx.Tx, a, b uuid.UUID) (uuid.UUID, error) {
	if a.String() > b.String() {
		a, b = b, a
	}
	var id uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO conversations (user_a, user_b) VALUES ($1,$2)
		 ON CONFLICT (user_a, user_b) DO NOTHING RETURNING id`, a, b).Scan(&id); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, err
		}
		// Already exists (possibly just committed by a concurrent caller,
		// which inserted the member rows in the same transaction).
		if err := tx.QueryRow(ctx,
			`SELECT id FROM conversations WHERE user_a=$1 AND user_b=$2`, a, b).Scan(&id); err != nil {
			return uuid.Nil, err
		}
		return id, nil
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO conversation_members (conversation_id, user_id) VALUES ($1,$2),($1,$3)
		 ON CONFLICT DO NOTHING`, id, a, b); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// CreateGuest creates a guest user (taking a slot) and its conversation with
// the developer.
func (s *Store) CreateGuest(ctx context.Context, developerID uuid.UUID, displayName string) (User, uuid.UUID, error) {
	for attempt := 0; attempt < 5; attempt++ {
		var (
			u    User
			conv uuid.UUID
		)
		err := s.WithTx(ctx, func(tx pgx.Tx) error {
			if err := takeSlot(ctx, tx); err != nil {
				return err
			}
			b := make([]byte, 2)
			if _, err := rand.Read(b); err != nil {
				return err
			}
			suffix := hex.EncodeToString(b)
			var err error
			u, err = scanUser(tx.QueryRow(ctx,
				`INSERT INTO users (kind, username, display_name) VALUES ('guest',$1,$2) RETURNING `+userCols,
				"guest-"+suffix, displayName))
			if err != nil {
				return err
			}
			conv, err = createConversationTx(ctx, tx, u.ID, developerID)
			return err
		})
		if pgCode(err) == "23505" { // username collision; try another suffix
			continue
		}
		return u, conv, err
	}
	return User{}, uuid.Nil, errors.New("create guest: could not allocate username")
}

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id=$1`, id))
}

func (s *Store) GetDeveloper(ctx context.Context) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE is_developer ORDER BY created_at LIMIT 1`))
}

// SetDisplayName sets the display name (validated by the caller).
func (s *Store) SetDisplayName(ctx context.Context, userID uuid.UUID, name string) (User, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`UPDATE users SET display_name=$2 WHERE id=$1 RETURNING `+userCols, userID, name))
}

// DeletedConversation is a conversation removed along with an account, and
// the peer who was in it.
type DeletedConversation struct {
	ConversationID uuid.UUID
	PeerID         uuid.UUID
}

// DeleteAccount deletes the user (cascading sessions, conversations and
// messages) and frees its slot, in one transaction. It returns the removed
// conversations with their peers; ErrNotFound if the user does not exist, is
// deactivated, or is a guest (guests end through EndGuest).
func (s *Store) DeleteAccount(ctx context.Context, userID uuid.UUID) ([]DeletedConversation, error) {
	var peers []DeletedConversation
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		peers = nil
		// Lock the user first; guests are refused so EndGuest's
		// conversation-then-user lock order is never inverted here.
		var kind string
		err := tx.QueryRow(ctx,
			`SELECT kind FROM users WHERE id=$1 AND deactivated_at IS NULL FOR UPDATE`, userID).Scan(&kind)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && kind == "guest") {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx,
			`SELECT id, CASE WHEN user_a=$1 THEN user_b ELSE user_a END
			 FROM conversations WHERE user_a=$1 OR user_b=$1`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var d DeletedConversation
			if err := rows.Scan(&d.ConversationID, &d.PeerID); err != nil {
				rows.Close()
				return err
			}
			peers = append(peers, d)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1 AND deactivated_at IS NULL`, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(ctx, `UPDATE user_slots SET used = used - 1 WHERE id = 1`)
		return err
	})
	if err != nil {
		return nil, err
	}
	return peers, nil
}
