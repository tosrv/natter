package chat

import (
	"context"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
)

// GuestNotifier is told how a guest ended so online peers can be informed
// (conversation_deleted or conversation_ended) and the guest disconnected.
type GuestNotifier interface {
	NotifyGuestEnded(outcome store.EndGuestOutcome, convs []store.EndedConversation, guestID uuid.UUID)
}

// EndGuest ends an active guest: deleted if it never sent a message,
// deactivated otherwise. not_found if guestID is not an active guest.
func (s *Service) EndGuest(ctx context.Context, guestID uuid.UUID) (store.EndGuestOutcome, []store.EndedConversation, error) {
	out, convs, err := s.st.EndGuest(ctx, guestID)
	return out, convs, mapErr(err)
}

// peerEnded reports whether the user peerID has been deactivated.
func (s *Service) peerEnded(ctx context.Context, peerID uuid.UUID) (bool, error) {
	p, err := s.st.GetUser(ctx, peerID)
	if err != nil {
		return false, mapErr(err)
	}
	return p.DeactivatedAt != nil, nil
}
