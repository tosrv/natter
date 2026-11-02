// Command loadtest drives N paired WebSocket clients against a running server
// and reports delivery latency, missing and duplicate messages.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/realtime"
	"github.com/tosrv/pmess/internal/store"
)

type client struct {
	idx    int
	userID uuid.UUID
	token  string
	conv   uuid.UUID
	peer   *client

	mu       sync.Mutex
	sentAt   map[string]time.Time // token -> send time
	received map[uuid.UUID]bool   // message ids received
	lats     []time.Duration
	nSent    int
	sendErrs int
	nRecv    int
	dups     int
	got      map[string]bool
}

func main() {
	url := flag.String("url", "ws://localhost:8080/api/ws", "websocket URL")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "database URL")
	clients := flag.Int("clients", 50, "number of clients (even)")
	messages := flag.Int("messages", 20, "messages per client")
	interval := flag.Duration("interval", 600*time.Millisecond, "per-client send interval")
	grace := flag.Duration("grace", 5*time.Second, "grace after last send")
	flag.Parse()
	if *clients < 2 || *clients%2 != 0 {
		fmt.Fprintln(os.Stderr, "-clients must be even and >= 2")
		os.Exit(2)
	}
	os.Exit(run(*url, *dsn, *clients, *messages, *interval, *grace))
}

func run(url, dsn string, n, msgs int, interval, grace time.Duration) int {
	ctx := context.Background()
	st, err := store.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "store:", err)
		return 1
	}
	defer st.Close()

	rb := make([]byte, 4)
	_, _ = rand.Read(rb)
	runID := hex.EncodeToString(rb)
	prefix := "loadtest-" + runID + "-"
	defer func() {
		// Best-effort cleanup; loadtest users hold user slots.
		tag, err := st.Pool().Exec(ctx, `DELETE FROM users WHERE google_sub LIKE $1`, prefix+"%")
		if err != nil {
			fmt.Fprintln(os.Stderr, "cleanup:", err)
			return
		}
		if k := tag.RowsAffected(); k > 0 {
			_, err = st.Pool().Exec(ctx, `UPDATE user_slots SET used = GREATEST(used - $1, 0) WHERE id = 1`, int(k))
			if err != nil {
				fmt.Fprintln(os.Stderr, "cleanup slots:", err)
			}
		}
	}()

	cs := make([]*client, n)
	for i := range cs {
		sub := fmt.Sprintf("%s%d", prefix, i)
		u, _, err := st.UpsertGoogleUser(ctx, sub, sub+"@loadtest.invalid", false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create user %d: %v\n", i, err)
			return 1
		}
		if _, err := st.SetUsername(ctx, u.ID, fmt.Sprintf("lt%s_%d", runID[:4], i)); err != nil {
			fmt.Fprintf(os.Stderr, "set username %d: %v\n", i, err)
			return 1
		}
		tok, err := st.CreateSession(ctx, u.ID, time.Hour)
		if err != nil {
			fmt.Fprintf(os.Stderr, "session %d: %v\n", i, err)
			return 1
		}
		cs[i] = &client{idx: i, userID: u.ID, token: tok,
			sentAt: map[string]time.Time{}, received: map[uuid.UUID]bool{}, got: map[string]bool{}}
	}
	for i := 0; i < n; i += 2 {
		conv, err := st.GetOrCreateConversation(ctx, cs[i].userID, cs[i+1].userID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "conversation:", err)
			return 1
		}
		cs[i].conv, cs[i+1].conv = conv, conv
		cs[i].peer, cs[i+1].peer = cs[i+1], cs[i]
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	conns := make([]*websocket.Conn, n)
	for i, c := range cs {
		h := http.Header{}
		h.Set("Authorization", "Bearer "+c.token)
		// Per-client IP so the per-IP rate limit does not couple clients
		// (honoured because the server trusts loopback proxies).
		h.Set("X-Real-IP", fmt.Sprintf("10.%d.%d.%d", 1+i/65025, (i/255)%255, 1+i%255))
		dctx, dc := context.WithTimeout(runCtx, 10*time.Second)
		ws, _, err := websocket.Dial(dctx, url, &websocket.DialOptions{HTTPHeader: h})
		dc()
		if err != nil {
			fmt.Fprintf(os.Stderr, "dial client %d: %v\n", i, err)
			return 1
		}
		ws.SetReadLimit(1 << 20)
		conns[i] = ws
	}
	defer func() {
		for _, ws := range conns {
			ws.Close(websocket.StatusNormalClosure, "done")
		}
	}()

	var readers sync.WaitGroup
	for i, c := range cs {
		readers.Add(1)
		go func(c *client, ws *websocket.Conn) {
			defer readers.Done()
			for {
				_, b, err := ws.Read(runCtx)
				if err != nil {
					return
				}
				now := time.Now()
				var f realtime.Frame
				if json.Unmarshal(b, &f) != nil {
					continue
				}
				if f.Type == "error" {
					fmt.Fprintf(os.Stderr, "client %d error frame: %s\n", c.idx, b)
					continue
				}
				if f.Type != "message" || f.Message == nil || f.Message.SenderID == c.userID {
					continue
				}
				c.onMessage(f.Message, now)
			}
		}(c, conns[i])
	}

	var senders sync.WaitGroup
	start := time.Now()
	for i, c := range cs {
		senders.Add(1)
		go func(c *client, ws *websocket.Conn) {
			defer senders.Done()
			tk := time.NewTicker(interval)
			defer tk.Stop()
			for k := 0; k < msgs; k++ {
				<-tk.C
				tok := fmt.Sprintf("%d-%d", c.idx, k)
				cid := uuid.New()
				fr, _ := json.Marshal(realtime.Frame{Type: "send", ConversationID: &c.conv, ClientMsgID: &cid, Body: tok})
				// Register before write so the peer's receipt can't race the timestamp.
				c.peer.mu.Lock()
				c.peer.sentAt[tok] = time.Now()
				c.peer.mu.Unlock()
				wctx, wc := context.WithTimeout(runCtx, 5*time.Second)
				err := ws.Write(wctx, websocket.MessageText, fr)
				wc()
				if err != nil {
					fmt.Fprintf(os.Stderr, "client %d write: %v\n", c.idx, err)
					c.mu.Lock()
					c.sendErrs += msgs - k
					c.mu.Unlock()
					return
				}
				c.mu.Lock()
				c.nSent++
				c.mu.Unlock()
			}
		}(c, conns[i])
	}
	senders.Wait()

	// Wait until everything arrived or the grace period ends.
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if tt := totals(cs); tt.recv >= tt.sent {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	readers.Wait()

	t := totals(cs)
	missing := t.sent - t.recv
	if missing < 0 {
		missing = 0
	}
	sort.Slice(t.lats, func(a, b int) bool { return t.lats[a] < t.lats[b] })
	pct := func(p float64) time.Duration {
		if len(t.lats) == 0 {
			return 0
		}
		return t.lats[int(float64(len(t.lats)-1)*p)]
	}
	p50, p95, max := pct(0.5), pct(0.95), pct(1)
	fmt.Printf("clients=%d messages=%d elapsed=%s\n", n, msgs, time.Since(start).Round(time.Millisecond))
	fmt.Printf("sent=%d received=%d missing=%d duplicates=%d p50=%s p95=%s max=%s sendErrors=%d\n",
		t.sent, t.recv, missing, t.dups, p50, p95, max, t.sendErrs)
	if t.sendErrs > 0 || t.sent != n*msgs || missing > 0 || t.dups > 0 || p95 >= time.Second {
		fmt.Println("FAIL")
		return 1
	}
	fmt.Println("OK")
	return 0
}

func (c *client) onMessage(m *store.Message, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.received[m.ID] {
		c.dups++
		return
	}
	c.received[m.ID] = true
	if c.got[m.Body] {
		c.dups++ // same token under a different id
		return
	}
	c.got[m.Body] = true
	c.nRecv++
	if t0, ok := c.sentAt[m.Body]; ok && !strings.HasPrefix(m.Body, fmt.Sprintf("%d-", c.idx)) {
		c.lats = append(c.lats, now.Sub(t0))
	}
}

type tot struct {
	sent, recv, dups, sendErrs int
	lats                       []time.Duration
}

func totals(cs []*client) tot {
	var t tot
	for _, c := range cs {
		c.mu.Lock()
		t.sent += c.nSent
		t.recv += c.nRecv
		t.dups += c.dups
		t.sendErrs += c.sendErrs
		t.lats = append(t.lats, c.lats...)
		c.mu.Unlock()
	}
	return t
}
