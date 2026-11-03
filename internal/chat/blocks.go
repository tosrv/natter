package chat

import (
	"context"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/store"
)

// notBlocked returns ErrNotFound when a and b are block-related.
func (s *Service) notBlocked(ctx context.Context, a, b uuid.UUID) error {
	rel, err := s.st.BlockRelated(ctx, a, b)
	if err != nil {
		return err
	}
	if rel {
		return ErrNotFound
	}
	return nil
}

// Block makes u block target. Guests cannot block; self → bad_request;
// unknown target → not_found. Idempotent.
func (s *Service) Block(ctx context.Context, u store.User, target uuid.UUID) error {
	if u.Kind == kindGuest {
		return ErrForbidden
	}
	if target == u.ID {
		return ErrBadRequest
	}
	return mapErr(s.st.Block(ctx, u.ID, target))
}

// Unblock removes u's block on target. Idempotent.
func (s *Service) Unblock(ctx context.Context, u store.User, target uuid.UUID) error {
	if u.Kind == kindGuest {
		return ErrForbidden
	}
	return mapErr(s.st.Unblock(ctx, u.ID, target))
}

// Blocks lists the users u has blocked, newest first.
func (s *Service) Blocks(ctx context.Context, u store.User) ([]store.BlockedUser, error) {
	if u.Kind == kindGuest {
		return nil, ErrForbidden
	}
	bl, err := s.st.ListBlocks(ctx, u.ID)
	return bl, mapErr(err)
}
