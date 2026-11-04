package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/api"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/config"
	"github.com/tosrv/natter/internal/ratelimit"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

const devEmail = "dev@example.com"

type env struct {
	st  *store.Store
	svc *chat.Service
	hub *Hub
	srv *httptest.Server
}

func setup(t *testing.T, msgLimit int, tweak func(*Hub)) *env {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	hub := NewHub(svc, ratelimit.New(msgLimit, 10*time.Second, nil), []string{"http://ok.example"})
	if tweak != nil {
		tweak(hub)
	}
	r := api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil), WS: hub.Handler()})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &env{st, svc, hub, srv}
}

func (e *env) user(t *testing.T, sub, username string) (store.User, string) {
	t.Helper()
	ctx := context.Background()
	u, err := e.svc.LoginGoogle(ctx, sub, sub+"@x.com", "")
	if err != nil {
		t.Fatal(err)
	}
	if u, err = e.svc.SetUsername(ctx, u, username); err != nil {
		t.Fatal(err)
	}
	tok, err := e.st.CreateSession(ctx, u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return u, tok
}

func (e *env) pair(t *testing.T) (a, b store.User, ta, tb string, conv uuid.UUID) {
	t.Helper()
	a, ta = e.user(t, "a", "alice")
	b, tb = e.user(t, "b", "bob")
	conv, err := e.svc.OpenConversation(context.Background(), a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func (e *env) dial(t *testing.T, tok string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Authorization", "Bearer "+tok)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatal(err)
	}
	c.SetReadLimit(1 << 20)
	t.Cleanup(func() { c.CloseNow() })
	return c
}

type rframe struct {
	Type           string          `json:"type"`
	ConversationID *uuid.UUID      `json:"conversation_id"`
	ClientMsgID    *uuid.UUID      `json:"client_msg_id"`
	UserID         *uuid.UUID      `json:"user_id"`
	Seq            int64           `json:"seq"`
	Code           string          `json:"code"`
	Message        json.RawMessage `json:"message"`
}

func (f rframe) msg(t *testing.T) store.Message {
	t.Helper()
	var m store.Message
	if err := json.Unmarshal(f.Message, &m); err != nil {
		t.Fatalf("message %s: %v", f.Message, err)
	}
	return m
}

func write(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func readFrame(t *testing.T, c *websocket.Conn, d time.Duration) (rframe, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	_, b, err := c.Read(ctx)
	if err != nil {
		return rframe{}, err
	}
	var f rframe
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("frame %s: %v", b, err)
	}
	return f, nil
}

func must(t *testing.T, c *websocket.Conn, typ string) rframe {
	t.Helper()
	f, err := readFrame(t, c, 2*time.Second)
	if err != nil {
		t.Fatalf("want %s: %v", typ, err)
	}
	if f.Type != typ {
		t.Fatalf("want %s, got %+v", typ, f)
	}
	return f
}

func send(conv, cid uuid.UUID, body string) map[string]any {
	return map[string]any{"type": "send", "conversation_id": conv, "client_msg_id": cid, "body": body}
}

// waitClosed reads until the connection errors; fails if it stays open.
func waitClosed(t *testing.T, c *websocket.Conn, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	for {
		if _, _, err := c.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("connection not closed")
			}
			return
		}
	}
}

// waitRegistered polls until the hub holds a connection for userID that is
// not prev, and returns it.
func waitRegistered(t *testing.T, h *Hub, userID uuid.UUID, prev *conn) *conn {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c := h.get(userID); c != nil && c != prev {
			return c
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("connection not registered")
	return nil
}

func TestDeliverOnlineUnder1s(t *testing.T) {
	e := setup(t, 20, nil)
	a, _, ta, tb, conv := e.pair(t)
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	cid := uuid.New()
	start := time.Now()
	write(t, ca, send(conv, cid, "halo"))
	f, err := readFrame(t, cb, time.Second)
	if err != nil || f.Type != "message" {
		t.Fatalf("B: %+v %v", f, err)
	}
	if time.Since(start) >= time.Second {
		t.Fatal("too slow")
	}
	m := f.msg(t)
	if m.Body != "halo" || m.SenderID != a.ID || m.ConversationID != conv || m.Seq != 1 {
		t.Fatalf("bad message %+v", m)
	}
	ack := must(t, ca, "ack")
	if ack.ClientMsgID == nil || *ack.ClientMsgID != cid || ack.msg(t).ID != m.ID {
		t.Fatalf("bad ack %+v", ack)
	}
}

func TestDuplicateNotRedelivered(t *testing.T) {
	e := setup(t, 20, nil)
	_, _, ta, tb, conv := e.pair(t)
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	cid := uuid.New()
	write(t, ca, send(conv, cid, "x"))
	id1 := must(t, ca, "ack").msg(t).ID
	write(t, ca, send(conv, cid, "x"))
	if id2 := must(t, ca, "ack").msg(t).ID; id2 != id1 {
		t.Fatalf("ack id %v != %v", id2, id1)
	}
	must(t, cb, "message")
	// a sentinel typing frame proves no second message is queued before it
	write(t, ca, map[string]any{"type": "typing", "conversation_id": conv})
	must(t, cb, "typing")
}

func TestTypingAndRead(t *testing.T) {
	e := setup(t, 20, nil)
	a, b, ta, tb, conv := e.pair(t)
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	write(t, ca, map[string]any{"type": "typing", "conversation_id": conv})
	f := must(t, cb, "typing")
	if *f.ConversationID != conv || *f.UserID != a.ID {
		t.Fatalf("typing %+v", f)
	}
	write(t, ca, send(conv, uuid.New(), "m"))
	must(t, ca, "ack")
	must(t, cb, "message")
	write(t, cb, map[string]any{"type": "read", "conversation_id": conv, "seq": 1})
	r := must(t, ca, "read")
	if *r.ConversationID != conv || *r.UserID != b.ID || r.Seq != 1 {
		t.Fatalf("read %+v", r)
	}
	// non-member typing -> error, connection stays open
	write(t, ca, map[string]any{"type": "typing", "conversation_id": uuid.New()})
	if f := must(t, ca, "error"); f.Code != "not_found" {
		t.Fatalf("code %q", f.Code)
	}
	write(t, ca, map[string]any{"type": "bogus"})
	must(t, ca, "error")
	write(t, ca, send(conv, uuid.New(), strings.Repeat("a", 2001)))
	if f := must(t, ca, "error"); f.Code != "message_too_long" || f.ClientMsgID == nil {
		t.Fatalf("too long %+v", f)
	}
}

func TestSecondConnectionReplacesFirst(t *testing.T) {
	e := setup(t, 20, nil)
	_, b, ta, tb, conv := e.pair(t)
	ca := e.dial(t, ta)
	cb1 := e.dial(t, tb)
	// Dial returns after the HTTP upgrade; the server registers the connection
	// a moment later in its handler goroutine. Wait for each registration so
	// "second" really means registered second (slow CI runners reorder them).
	first := waitRegistered(t, e.hub, b.ID, nil)
	cb2 := e.dial(t, tb)
	waitRegistered(t, e.hub, b.ID, first)
	waitClosed(t, cb1, 2*time.Second)
	write(t, ca, send(conv, uuid.New(), "hi"))
	must(t, cb2, "message")
}

func TestSlowConsumerDropped(t *testing.T) {
	block := make(chan struct{})
	var bID uuid.UUID
	e := setup(t, 1000, func(h *Hub) {
		h.beforeWrite = func(u uuid.UUID) {
			if u == bID {
				<-block
			}
		}
	})
	defer close(block)
	_, b, ta, tb, conv := e.pair(t)
	bID = b.ID
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	for i := 0; i < 200; i++ {
		write(t, ca, send(conv, uuid.New(), "m"))
		must(t, ca, "ack")
	}
	waitClosed(t, cb, 3*time.Second)
	ctx := context.Background()
	older, err1 := e.st.ListMessages(ctx, conv, b.ID, 0, 101, 100) // seq 1..100
	newer, err2 := e.st.ListMessages(ctx, conv, b.ID, 100, 0, 100) // seq 101..200
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	total := len(older) + len(newer)
	if total != 200 {
		t.Fatalf("db has %d msgs", total)
	}
	if e.hub.online(b.ID) {
		t.Fatal("B still registered")
	}
}

func TestOversizeFrameClosed(t *testing.T) {
	e := setup(t, 20, nil)
	_, _, ta, _, conv := e.pair(t)
	ca := e.dial(t, ta)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, _ := json.Marshal(send(conv, uuid.New(), strings.Repeat("a", 9000)))
	_ = ca.Write(ctx, websocket.MessageText, b)
	waitClosed(t, ca, 2*time.Second)
}

func TestRateLimitedFrame(t *testing.T) {
	e := setup(t, 20, nil)
	_, _, ta, _, conv := e.pair(t)
	ca := e.dial(t, ta)
	for i := 0; i < 20; i++ {
		write(t, ca, send(conv, uuid.New(), "m"))
		must(t, ca, "ack")
	}
	cid := uuid.New()
	write(t, ca, send(conv, cid, "m"))
	f := must(t, ca, "error")
	if f.Code != "rate_limited" || f.ClientMsgID == nil || *f.ClientMsgID != cid {
		t.Fatalf("got %+v", f)
	}
	var m string
	_ = json.Unmarshal(f.Message, &m)
	if m == "" {
		t.Fatal("error message empty")
	}
}

func TestDisconnectDeletedGuest(t *testing.T) {
	e := setup(t, 20, nil)
	ctx := context.Background()
	if _, err := e.svc.LoginGoogle(ctx, "devsub", devEmail, ""); err != nil {
		t.Fatal(err)
	}
	g, _, err := e.svc.CreateGuest(ctx, "Guest Test")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := e.st.CreateSession(ctx, g.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cg := e.dial(t, tok)
	deadline := time.Now().Add(2 * time.Second)
	for !e.hub.online(g.ID) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if _, _, err := e.st.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	e.hub.Disconnect(g.ID)
	waitClosed(t, cg, 2*time.Second)
	if e.hub.online(g.ID) {
		t.Fatal("guest still registered")
	}
}

func TestPingTimeoutCloses(t *testing.T) {
	e := setup(t, 20, func(h *Hub) { h.PingInterval = 20 * time.Millisecond; h.PongTimeout = 50 * time.Millisecond })
	_, _, ta, _, _ := e.pair(t)
	ca := e.dial(t, ta)
	// client never reads -> pongs never processed -> server closes
	time.Sleep(300 * time.Millisecond)
	waitClosed(t, ca, 2*time.Second)
}

func TestBadOriginRejected(t *testing.T) {
	e := setup(t, 20, nil)
	_, _, ta, _, _ := e.pair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Authorization", "Bearer "+ta)
	h.Set("Origin", "http://evil.example")
	if _, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPHeader: h}); err == nil {
		t.Fatal("evil origin accepted")
	}
	h.Set("Origin", "http://ok.example")
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatal(err)
	}
	c.CloseNow()
}

func TestUnauthenticatedRejected(t *testing.T) {
	e := setup(t, 20, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http")+"/api/ws", nil); err == nil {
		t.Fatal("unauthenticated accepted")
	}
}

func TestNotifyConversationDeleted(t *testing.T) {
	e := setup(t, 20, nil)
	a, b, ta, tb, conv := e.pair(t)
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	deadline := time.Now().Add(2 * time.Second)
	for !(e.hub.online(a.ID) && e.hub.online(b.ID)) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	offline := uuid.New()
	e.hub.NotifyConversationDeleted([]store.DeletedConversation{
		{ConversationID: conv, PeerID: b.ID},
		{ConversationID: uuid.New(), PeerID: offline},
	}, a.ID)
	f := must(t, cb, "conversation_deleted")
	if f.ConversationID == nil || *f.ConversationID != conv {
		t.Fatalf("conversation_id = %v, want %v", f.ConversationID, conv)
	}
	waitClosed(t, ca, 2*time.Second)
}

func TestTypingAndReadSuppressedWhenBlocked(t *testing.T) {
	e := setup(t, 20, nil)
	a, b, ta, tb, conv := e.pair(t)
	_ = a
	if err := e.svc.Block(context.Background(), a, b.ID); err != nil {
		t.Fatal(err)
	}
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	write(t, cb, map[string]any{"type": "typing", "conversation_id": conv})
	write(t, cb, map[string]any{"type": "read", "conversation_id": conv, "seq": 1})
	if f, err := readFrame(t, ca, 500*time.Millisecond); err == nil {
		t.Fatalf("A got frame %+v", f)
	}
	if f, err := readFrame(t, cb, 500*time.Millisecond); err == nil {
		t.Fatalf("B got frame %+v", f)
	}
	_ = ca
}

func TestSendBlockedErrorFrame(t *testing.T) {
	e := setup(t, 20, nil)
	a, b, ta, tb, conv := e.pair(t)
	if err := e.svc.Block(context.Background(), a, b.ID); err != nil {
		t.Fatal(err)
	}
	ca, cb := e.dial(t, ta), e.dial(t, tb)
	cid := uuid.New()
	write(t, cb, send(conv, cid, "hi"))
	f := must(t, cb, "error")
	if f.Code != "not_found" || f.ClientMsgID == nil || *f.ClientMsgID != cid {
		t.Fatalf("frame %+v", f)
	}
	if f, err := readFrame(t, ca, 500*time.Millisecond); err == nil {
		t.Fatalf("A got frame %+v", f)
	}
}

func TestNotifyGuestEnded(t *testing.T) {
	e := setup(t, 20, nil)
	ctx := context.Background()
	dev, err := e.svc.LoginGoogle(ctx, "devsub", devEmail, "")
	if err != nil {
		t.Fatal(err)
	}
	dtok, err := e.st.CreateSession(ctx, dev.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	g, conv, err := e.svc.CreateGuest(ctx, "Guest Test")
	if err != nil {
		t.Fatal(err)
	}
	gtok, err := e.st.CreateSession(ctx, g.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cd, cg := e.dial(t, dtok), e.dial(t, gtok)
	deadline := time.Now().Add(2 * time.Second)
	for !(e.hub.online(dev.ID) && e.hub.online(g.ID)) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	e.hub.NotifyGuestEnded(store.GuestDeactivated, []store.EndedConversation{{ConversationID: conv, PeerID: dev.ID}}, g.ID)
	f := must(t, cd, "conversation_ended")
	if f.ConversationID == nil || *f.ConversationID != conv {
		t.Fatalf("conversation_id = %v, want %v", f.ConversationID, conv)
	}
	waitClosed(t, cg, 2*time.Second)

	other := uuid.New()
	e.hub.NotifyGuestEnded(store.GuestDeleted, []store.EndedConversation{{ConversationID: other, PeerID: dev.ID}}, uuid.New())
	f = must(t, cd, "conversation_deleted")
	if f.ConversationID == nil || *f.ConversationID != other {
		t.Fatalf("conversation_id = %v, want %v", f.ConversationID, other)
	}
}

func TestWSSubprotocolAuth(t *testing.T) {
	e := setup(t, 20, nil)
	_, _, ta, _, _ := e.pair(t)
	url := "ws" + strings.TrimPrefix(e.srv.URL, "http") + "/api/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Origin", "http://ok.example")
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h, Subprotocols: []string{auth.WSSubprotocol, ta}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if c.Subprotocol() != auth.WSSubprotocol {
		t.Fatalf("subprotocol %q", c.Subprotocol())
	}
	_, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h, Subprotocols: []string{auth.WSSubprotocol, "bogus"}})
	if err == nil {
		t.Fatal("bad token accepted")
	}
	if resp == nil || resp.StatusCode != 401 {
		t.Fatalf("resp %v", resp)
	}
}

func TestNotifyConversationCleared(t *testing.T) {
	e := setup(t, 20, nil)
	a, b, ta, tb, conv := e.pair(t)
	// The hub keeps one connection per user: a second dial replaces the first.
	ca1 := e.dial(t, ta)
	c1 := waitRegistered(t, e.hub, a.ID, nil)
	ca := e.dial(t, ta)
	waitRegistered(t, e.hub, a.ID, c1)
	_ = ca1
	cb := e.dial(t, tb)
	waitRegistered(t, e.hub, b.ID, nil)
	e.hub.NotifyConversationCleared(a.ID, conv)
	f := must(t, ca, "conversation_deleted")
	if f.ConversationID == nil || *f.ConversationID != conv {
		t.Fatalf("conversation_id = %v, want %v", f.ConversationID, conv)
	}
	if f, err := readFrame(t, cb, 300*time.Millisecond); err == nil {
		t.Fatalf("B got frame %+v", f)
	}
}
