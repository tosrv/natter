package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	SenderID       uuid.UUID `json:"sender_id"`
	Seq            int64     `json:"seq"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
	ClientMsgID    uuid.UUID `json:"client_msg_id,omitempty"`
}

const msgCols = `id, conversation_id, sender_id, seq, body, created_at, client_msg_id`

func scanMessage(row pgx.Row) (Message, error) {
	var m Message
	err := row.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Seq, &m.Body, &m.CreatedAt, &m.ClientMsgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	return m, err
}

var errDupInsert = errors.New("duplicate client_msg_id")

func (s *Store) existingMessage(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, senderID, clientMsgID uuid.UUID) (Message, error) {
	return scanMessage(q.QueryRow(ctx,
		`SELECT `+msgCols+` FROM messages WHERE sender_id=$1 AND client_msg_id=$2`, senderID, clientMsgID))
}

// SendMessage stores a message with the next per-conversation seq. It is
// idempotent on (sender_id, client_msg_id): a repeat returns the original
// message with duplicate=true and does not consume a seq. Membership is the
// caller's responsibility. Unknown conversation → ErrNotFound.
func (s *Store) SendMessage(ctx context.Context, convID, senderID, clientMsgID uuid.UUID, body string) (Message, bool, error) {
	var (
		m   Message
		dup bool
	)
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		old, err := s.existingMessage(ctx, tx, senderID, clientMsgID)
		if err == nil {
			m, dup = old, true
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		var seq int64
		err = tx.QueryRow(ctx,
			`UPDATE conversations SET last_seq=last_seq+1, last_activity_at=now() WHERE id=$1 RETURNING last_seq`,
			convID).Scan(&seq)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// A sender deactivated while we waited on the conversation lock
		// (EndGuest) must not persist a message; rolling back keeps last_seq.
		var one int
		err = tx.QueryRow(ctx,
			`SELECT 1 FROM users WHERE id=$1 AND deactivated_at IS NULL FOR KEY SHARE`, senderID).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		m, err = scanMessage(tx.QueryRow(ctx,
			`INSERT INTO messages (conversation_id, seq, sender_id, client_msg_id, body)
			 VALUES ($1,$2,$3,$4,$5) RETURNING `+msgCols, convID, seq, senderID, clientMsgID, body))
		if pgCode(err) == "23505" {
			return errDupInsert // roll back so last_seq is not consumed
		}
		return err
	})
	if errors.Is(err, errDupInsert) {
		m, err = s.existingMessage(ctx, s.pool, senderID, clientMsgID)
		if err != nil {
			return Message{}, false, err
		}
		return m, true, nil
	}
	if err != nil {
		return Message{}, false, err
	}
	return m, dup, nil
}

// ListMessages pages a conversation. afterSeq>0: messages with seq>afterSeq
// ascending. Otherwise the newest messages (below beforeSeq if >0), returned
// ascending. limit is clamped to 1..100.
func (s *Store) ListMessages(ctx context.Context, convID uuid.UUID, afterSeq, beforeSeq int64, limit int) ([]Message, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	var (
		rows pgx.Rows
		err  error
	)
	if afterSeq > 0 {
		rows, err = s.pool.Query(ctx, `SELECT `+msgCols+` FROM messages
			WHERE conversation_id=$1 AND seq>$2 ORDER BY seq ASC LIMIT $3`, convID, afterSeq, limit)
	} else {
		if beforeSeq <= 0 {
			beforeSeq = 1<<63 - 1
		}
		rows, err = s.pool.Query(ctx, `SELECT * FROM (SELECT `+msgCols+` FROM messages
			WHERE conversation_id=$1 AND seq<$2 ORDER BY seq DESC LIMIT $3) t ORDER BY seq ASC`,
			convID, beforeSeq, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkRead advances userID's read pointer monotonically, capped at last_seq,
// and returns the new value. Non-member → ErrNotFound.
func (s *Store) MarkRead(ctx context.Context, convID, userID uuid.UUID, seq int64) (int64, error) {
	var v int64
	err := s.pool.QueryRow(ctx, `UPDATE conversation_members cm
		SET last_read_seq = GREATEST(cm.last_read_seq, LEAST($3, c.last_seq))
		FROM conversations c
		WHERE c.id = cm.conversation_id AND cm.conversation_id=$1 AND cm.user_id=$2
		RETURNING cm.last_read_seq`, convID, userID, seq).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return v, err
}
