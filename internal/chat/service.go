package chat

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/store"
)

const (
	maxBodyRunes = 2000
	minQueryLen  = 2
	searchLimit  = 10
	kindGuest    = "guest"
	kindGoogle   = "google"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

type Service struct {
	st             *store.Store
	developerEmail string
}

func New(st *store.Store, developerEmail string) *Service {
	return &Service{st: st, developerEmail: developerEmail}
}

func (s *Service) LoginGoogle(ctx context.Context, sub, email, name string) (store.User, error) {
	display, err := ValidateDisplayName(name)
	if err != nil {
		display = "User"
	}
	isDev := s.developerEmail != "" && strings.EqualFold(email, s.developerEmail)
	u, _, err := s.st.UpsertGoogleUser(ctx, sub, email, display, isDev)
	return u, mapErr(err)
}

// CreateGuest creates a guest and its conversation with the developer.
// The developer is looked up first so no slot is consumed if absent.
func (s *Service) CreateGuest(ctx context.Context) (store.User, uuid.UUID, error) {
	dev, err := s.st.GetDeveloper(ctx)
	if err != nil {
		if mapErr(err) == ErrNotFound {
			return store.User{}, uuid.Nil, ErrDeveloperUnavailable
		}
		return store.User{}, uuid.Nil, err
	}
	u, conv, err := s.st.CreateGuest(ctx, dev.ID)
	return u, conv, mapErr(err)
}

func (s *Service) SetUsername(ctx context.Context, u store.User, raw string) (store.User, error) {
	if u.Kind == kindGuest {
		return store.User{}, ErrForbidden
	}
	name := strings.ToLower(strings.TrimSpace(raw))
	if !usernameRe.MatchString(name) {
		return store.User{}, ErrUsernameInvalid
	}
	nu, err := s.st.SetUsername(ctx, u.ID, name)
	return nu, mapErr(err)
}

// ready enforces that a Google user has chosen a username.
func ready(u store.User) error {
	if u.Kind != kindGuest && (u.Username == nil || *u.Username == "") {
		return ErrUsernameRequired
	}
	return nil
}

func (s *Service) Search(ctx context.Context, u store.User, q string) ([]store.UserBrief, error) {
	if u.Kind == kindGuest {
		return nil, ErrForbidden
	}
	if err := ready(u); err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	if utf8.RuneCountInString(q) < minQueryLen {
		return nil, ErrQueryTooShort
	}
	res, err := s.st.SearchUsers(ctx, q, u.ID, searchLimit)
	return res, mapErr(err)
}

func (s *Service) OpenConversation(ctx context.Context, u store.User, peerID uuid.UUID) (uuid.UUID, error) {
	if u.Kind == kindGuest {
		return uuid.Nil, ErrForbidden
	}
	if err := ready(u); err != nil {
		return uuid.Nil, err
	}
	if peerID == u.ID {
		return uuid.Nil, ErrNotFound
	}
	p, err := s.st.GetUser(ctx, peerID)
	if err != nil {
		return uuid.Nil, mapErr(err)
	}
	if p.Kind != kindGoogle || p.Username == nil || *p.Username == "" {
		return uuid.Nil, ErrNotFound
	}
	if err := s.notBlocked(ctx, u.ID, peerID); err != nil {
		return uuid.Nil, err
	}
	id, err := s.st.GetOrCreateConversation(ctx, u.ID, peerID)
	return id, mapErr(err)
}

// peer checks readiness and membership, returning the other member.
func (s *Service) peer(ctx context.Context, u store.User, convID uuid.UUID) (uuid.UUID, error) {
	if err := ready(u); err != nil {
		return uuid.Nil, err
	}
	p, err := s.st.Peer(ctx, convID, u.ID)
	return p, mapErr(err)
}

func (s *Service) Send(ctx context.Context, u store.User, convID, clientMsgID uuid.UUID, body string) (store.Message, uuid.UUID, bool, error) {
	if err := ready(u); err != nil {
		return store.Message{}, uuid.Nil, false, err
	}
	if strings.TrimSpace(body) == "" {
		return store.Message{}, uuid.Nil, false, ErrMessageEmpty
	}
	if utf8.RuneCountInString(body) > maxBodyRunes {
		return store.Message{}, uuid.Nil, false, ErrMessageTooLong
	}
	p, err := s.peer(ctx, u, convID)
	if err != nil {
		return store.Message{}, uuid.Nil, false, err
	}
	if err := s.notBlocked(ctx, u.ID, p); err != nil {
		return store.Message{}, uuid.Nil, false, err
	}
	m, dup, err := s.st.SendMessage(ctx, convID, u.ID, clientMsgID, body)
	if err != nil {
		return store.Message{}, uuid.Nil, false, mapErr(err)
	}
	return m, p, dup, nil
}

func (s *Service) Messages(ctx context.Context, u store.User, convID uuid.UUID, afterSeq, beforeSeq int64, limit int) ([]store.Message, error) {
	if _, err := s.peer(ctx, u, convID); err != nil {
		return nil, err
	}
	ms, err := s.st.ListMessages(ctx, convID, afterSeq, beforeSeq, limit)
	return ms, mapErr(err)
}

// MarkRead stores u's read position; notifyPeer is false when u and the
// peer are block-related (the read event must not be forwarded).
func (s *Service) MarkRead(ctx context.Context, u store.User, convID uuid.UUID, seq int64) (int64, uuid.UUID, bool, error) {
	p, err := s.peer(ctx, u, convID)
	if err != nil {
		return 0, uuid.Nil, false, err
	}
	v, err := s.st.MarkRead(ctx, convID, u.ID, seq)
	if err != nil {
		return 0, uuid.Nil, false, mapErr(err)
	}
	rel, err := s.st.BlockRelated(ctx, u.ID, p)
	if err != nil {
		return 0, uuid.Nil, false, err
	}
	return v, p, !rel, nil
}

func (s *Service) Conversations(ctx context.Context, u store.User) ([]store.ConversationSummary, error) {
	if err := ready(u); err != nil {
		return nil, err
	}
	cs, err := s.st.ListConversations(ctx, u.ID)
	return cs, mapErr(err)
}

// PeerOf returns the peer of convID; deliver is false when u and the peer
// are block-related (e.g. typing must not be forwarded).
func (s *Service) PeerOf(ctx context.Context, u store.User, convID uuid.UUID) (uuid.UUID, bool, error) {
	p, err := s.peer(ctx, u, convID)
	if err != nil {
		return uuid.Nil, false, err
	}
	rel, err := s.st.BlockRelated(ctx, u.ID, p)
	if err != nil {
		return uuid.Nil, false, err
	}
	return p, !rel, nil
}

func (s *Service) SetDisplayName(ctx context.Context, u store.User, raw string) (store.User, error) {
	if u.Kind == kindGuest {
		return store.User{}, ErrForbidden
	}
	name, err := ValidateDisplayName(raw)
	if err != nil {
		return store.User{}, err
	}
	nu, err := s.st.SetDisplayName(ctx, u.ID, name)
	return nu, mapErr(err)
}

// DeleteAccount removes the user (guests included); the developer is protected.
func (s *Service) DeleteAccount(ctx context.Context, u store.User) ([]store.DeletedConversation, error) {
	if u.IsDeveloper {
		return nil, ErrForbidden
	}
	d, err := s.st.DeleteAccount(ctx, u.ID)
	return d, mapErr(err)
}
