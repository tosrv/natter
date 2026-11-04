package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// GetOrCreateConversation returns the unique conversation between a and b,
// creating it with both membership rows if needed. Safe under concurrency.
func (s *Store) GetOrCreateConversation(ctx context.Context, a, b uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		var err error
		id, err = createConversationTx(ctx, tx, a, b)
		return err
	})
	return id, err
}

func (s *Store) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`,
		convID, userID).Scan(&ok)
	return ok, err
}

// Peer returns the other member of convID; ErrNotFound if userID is not a member.
func (s *Store) Peer(ctx context.Context, convID, userID uuid.UUID) (uuid.UUID, error) {
	var peer uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT CASE WHEN user_a=$2 THEN user_b ELSE user_a END FROM conversations
		 WHERE id=$1 AND (user_a=$2 OR user_b=$2)`, convID, userID).Scan(&peer)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return peer, err
}

type ConversationSummary struct {
	ID              uuid.UUID `json:"id"`
	PeerID          uuid.UUID `json:"peer_id"`
	PeerUsername    string    `json:"peer_username"`
	PeerDisplayName string    `json:"peer_display_name"`
	PeerKind        string    `json:"peer_kind"`
	LastMessage     *Message  `json:"last_message"`
	Unread          int64     `json:"unread"`
	PeerLastReadSeq int64     `json:"peer_last_read_seq"`
	LastActivityAt  time.Time `json:"last_activity_at"`
	BlockedByMe     bool      `json:"blocked_by_me"`
	PeerEnded       bool      `json:"peer_ended"`
}

// ListConversations lists userID's conversations, most recently active first.
// LastActivityAt falls back to created_at for conversations without messages.
func (s *Store) ListConversations(ctx context.Context, userID uuid.UUID) ([]ConversationSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, p.id, coalesce(p.username,''), p.display_name, p.kind,
		       c.last_seq - GREATEST(me.last_read_seq, me.cleared_seq), pm.last_read_seq,
		       coalesce(c.last_activity_at, c.created_at),
		       m.id, m.seq, m.sender_id, m.body, m.created_at, m.client_msg_id,
		       EXISTS (SELECT 1 FROM blocks b WHERE b.blocker_id = me.user_id AND b.blocked_id = p.id),
		       p.deactivated_at IS NOT NULL
		FROM conversation_members me
		JOIN conversations c ON c.id = me.conversation_id
		JOIN conversation_members pm ON pm.conversation_id = c.id AND pm.user_id <> me.user_id
		JOIN users p ON p.id = pm.user_id
		LEFT JOIN messages m ON m.conversation_id = c.id AND m.seq = c.last_seq
		WHERE me.user_id = $1 AND (c.last_seq > me.cleared_seq OR me.cleared_seq = 0)
		ORDER BY coalesce(c.last_activity_at, c.created_at) DESC, c.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ConversationSummary{}
	for rows.Next() {
		var (
			cs                 ConversationSummary
			mID, mSender, mCli *uuid.UUID
			mSeq               *int64
			mBody              *string
			mAt                *time.Time
		)
		if err := rows.Scan(&cs.ID, &cs.PeerID, &cs.PeerUsername, &cs.PeerDisplayName, &cs.PeerKind,
			&cs.Unread, &cs.PeerLastReadSeq, &cs.LastActivityAt,
			&mID, &mSeq, &mSender, &mBody, &mAt, &mCli, &cs.BlockedByMe, &cs.PeerEnded); err != nil {
			return nil, err
		}
		if mID != nil {
			cs.LastMessage = &Message{ID: *mID, ConversationID: cs.ID, SenderID: *mSender,
				Seq: *mSeq, Body: *mBody, CreatedAt: *mAt, ClientMsgID: *mCli}
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

// ClearConversation hides the conversation's history for userID only (delete
// for me). Idempotent; non-member → ErrNotFound.
func (s *Store) ClearConversation(ctx context.Context, convID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE conversation_members cm
		SET cleared_seq = GREATEST(cm.cleared_seq, c.last_seq)
		FROM conversations c
		WHERE c.id = cm.conversation_id AND cm.conversation_id=$1 AND cm.user_id=$2`, convID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
