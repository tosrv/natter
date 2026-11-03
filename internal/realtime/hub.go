package realtime

import (
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store"
)

const maxFrameBytes = 8192

// Hub tracks one connection per user and routes frames between peers.
type Hub struct {
	svc        *chat.Service
	msgLimiter *ratelimit.Limiter
	origins    []string

	// Tunables (set before Handler is serving).
	PingInterval time.Duration
	PongTimeout  time.Duration
	OutBuffer    int

	beforeWrite func(uuid.UUID) // test hook

	mu    sync.RWMutex
	conns map[uuid.UUID]*conn
}

func NewHub(svc *chat.Service, msgLimiter *ratelimit.Limiter, allowedOrigins []string) *Hub {
	return &Hub{
		svc: svc, msgLimiter: msgLimiter, origins: originPatterns(allowedOrigins),
		PingInterval: 25 * time.Second, PongTimeout: 60 * time.Second, OutBuffer: 64,
		conns: map[uuid.UUID]*conn{},
	}
}

// register installs c, closing any previous connection of the same user.
func (h *Hub) register(c *conn) {
	h.mu.Lock()
	old := h.conns[c.userID]
	h.conns[c.userID] = c
	h.mu.Unlock()
	if old != nil {
		old.close("replaced by new connection")
	}
}

// unregister removes c only if it is still the registered connection.
func (h *Hub) unregister(c *conn) {
	h.mu.Lock()
	if h.conns[c.userID] == c {
		delete(h.conns, c.userID)
	}
	h.mu.Unlock()
}

func (h *Hub) get(userID uuid.UUID) *conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[userID]
}

func (h *Hub) online(userID uuid.UUID) bool { return h.get(userID) != nil }

// deliver enqueues f to userID if online; never blocks.
func (h *Hub) deliver(userID uuid.UUID, f Frame) {
	if c := h.get(userID); c != nil {
		c.enqueue(f)
	}
}

// Disconnect closes the user's connection (e.g. guest deleted).
func (h *Hub) Disconnect(userID uuid.UUID) {
	h.mu.Lock()
	c := h.conns[userID]
	delete(h.conns, userID)
	h.mu.Unlock()
	if c != nil {
		c.close("account removed")
	}
}

// NotifyConversationDeleted tells each online peer that a conversation was
// removed, then disconnects the deleted user. Never blocks.
func (h *Hub) NotifyConversationDeleted(peers []store.DeletedConversation, deletedUser uuid.UUID) {
	for _, p := range peers {
		id := p.ConversationID
		h.deliver(p.PeerID, Frame{Type: "conversation_deleted", ConversationID: &id})
	}
	h.Disconnect(deletedUser)
}

// NotifyGuestEnded tells each online peer how the guest's conversations
// ended: conversation_deleted for GuestDeleted, conversation_ended for
// GuestDeactivated. The guest is then disconnected. Never blocks.
func (h *Hub) NotifyGuestEnded(outcome store.EndGuestOutcome, convs []store.EndedConversation, guestID uuid.UUID) {
	typ := "conversation_ended"
	if outcome == store.GuestDeleted {
		typ = "conversation_deleted"
	}
	for _, c := range convs {
		id := c.ConversationID
		h.deliver(c.PeerID, Frame{Type: typ, ConversationID: &id})
	}
	h.Disconnect(guestID)
}

// originPatterns converts full origins ("https://host:port") into the host
// patterns websocket.AcceptOptions expects; bare hosts pass through.
func originPatterns(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			o = u.Host
		}
		out = append(out, o)
	}
	return out
}
