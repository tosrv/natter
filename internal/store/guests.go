package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// EndGuestOutcome says how EndGuest ended a guest.
type EndGuestOutcome int

const (
	// GuestDeleted: the guest had sent no messages and was removed entirely
	// (cascading its conversations).
	GuestDeleted EndGuestOutcome = iota + 1
	// GuestDeactivated: the guest had messages; the row is kept with
	// deactivated_at set, username cleared and sessions removed.
	GuestDeactivated
)

// EndedConversation is a conversation of an ended guest and its peer.
type EndedConversation struct {
	ConversationID uuid.UUID
	PeerID         uuid.UUID
}

// EndGuest ends an active guest in one transaction, freeing its slot exactly
// once. ErrNotFound if id is not an active guest (unknown, Google, already
// ended).
//
// Locking: the guest's conversation rows are locked before the user row.
// SendMessage locks the conversation row (UPDATE last_seq) and then takes a
// FOR KEY SHARE lock on the sender via the messages FK, so taking the locks in
// the same order serializes the two without deadlock: either the message
// commits first and is counted (→ deactivate), or SendMessage waits and finds
// the conversation deleted (→ ErrNotFound), never losing a stored message.
func (s *Store) EndGuest(ctx context.Context, id uuid.UUID) (EndGuestOutcome, []EndedConversation, error) {
	var (
		out   EndGuestOutcome
		convs []EndedConversation
	)
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		convs = nil
		rows, err := tx.Query(ctx,
			`SELECT id, CASE WHEN user_a=$1 THEN user_b ELSE user_a END
			 FROM conversations WHERE user_a=$1 OR user_b=$1 ORDER BY id FOR UPDATE`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var c EndedConversation
			if err := rows.Scan(&c.ConversationID, &c.PeerID); err != nil {
				rows.Close()
				return err
			}
			convs = append(convs, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		var locked uuid.UUID
		err = tx.QueryRow(ctx,
			`SELECT id FROM users WHERE id=$1 AND kind='guest' AND deactivated_at IS NULL FOR UPDATE`,
			id).Scan(&locked)
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var hasMsgs bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM messages WHERE sender_id=$1)`, id).Scan(&hasMsgs); err != nil {
			return err
		}
		if hasMsgs {
			out = GuestDeactivated
			if _, err := tx.Exec(ctx,
				`UPDATE users SET deactivated_at=now(), username=NULL WHERE id=$1`, id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, id); err != nil {
				return err
			}
		} else {
			out = GuestDeleted
			if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1`, id); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE user_slots SET used = used - 1 WHERE id = 1`)
		return err
	})
	if err != nil {
		return 0, nil, err
	}
	return out, convs, nil
}

// PurgeDeactivatedGuests removes deactivated guests that no longer have any
// conversation. Slots were already freed at deactivation, so used is unchanged.
func (s *Store) PurgeDeactivatedGuests(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM users u
		WHERE u.kind = 'guest'
		  AND u.deactivated_at IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM conversations c WHERE c.user_a = u.id OR c.user_b = u.id)`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
