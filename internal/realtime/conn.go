package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/store"
)

type conn struct {
	hub    *Hub
	ws     *websocket.Conn
	user   store.User
	userID uuid.UUID
	ip     string
	out    chan []byte
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

// Handler upgrades an authenticated request; requires auth.Middleware.
func (h *Hub) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{OriginPatterns: h.origins, Subprotocols: []string{auth.WSSubprotocol}})
		if err != nil {
			return // Accept already wrote the HTTP error
		}
		ws.SetReadLimit(maxFrameBytes)
		u := auth.CurrentUser(c)
		ctx, cancel := context.WithCancel(context.Background())
		cn := &conn{hub: h, ws: ws, user: u, userID: u.ID, ip: auth.ClientIP(c),
			out: make(chan []byte, h.OutBuffer), ctx: ctx, cancel: cancel}
		h.register(cn)
		go cn.writeLoop()
		go cn.pingLoop()
		cn.readLoop()
	}
}

func (c *conn) close(reason string) {
	c.once.Do(func() {
		c.cancel()
		c.hub.unregister(c)
		go c.ws.Close(websocket.StatusPolicyViolation, reason)
		// Close waits for the handshake; force teardown if the peer stalls.
		time.AfterFunc(5*time.Second, func() { c.ws.CloseNow() })
	})
}

// enqueue never blocks: a full buffer drops the (slow) connection.
func (c *conn) enqueue(f Frame) {
	b, err := json.Marshal(f)
	if err != nil {
		return
	}
	select {
	case <-c.ctx.Done():
	case c.out <- b:
	default:
		c.close("slow consumer")
	}
}

func (c *conn) writeLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case b := <-c.out:
			if c.hub.beforeWrite != nil {
				c.hub.beforeWrite(c.userID)
			}
			ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
			err := c.ws.Write(ctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				c.close("write failed")
				return
			}
		}
	}
}

func (c *conn) pingLoop() {
	t := time.NewTicker(c.hub.PingInterval)
	defer t.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(c.ctx, c.hub.PongTimeout)
			err := c.ws.Ping(ctx)
			cancel()
			if err != nil {
				c.close("pong timeout")
				return
			}
		}
	}
}

func (c *conn) readLoop() {
	defer c.close("closed")
	for {
		typ, b, err := c.ws.Read(c.ctx)
		if err != nil {
			return // includes oversize frames (read limit closes with 1009)
		}
		if typ != websocket.MessageText {
			c.sendErr(nil, "bad_frame", "text frames only")
			continue
		}
		var f Frame
		if err := json.Unmarshal(b, &f); err != nil {
			c.sendErr(nil, "bad_frame", "invalid JSON frame")
			continue
		}
		c.handle(f)
	}
}

func (c *conn) sendErr(clientMsgID *uuid.UUID, code, msg string) {
	c.enqueue(Frame{Type: "error", ClientMsgID: clientMsgID, Code: code, ErrMessage: msg})
}

func (c *conn) svcErr(clientMsgID *uuid.UUID, err error) {
	var ce *chat.Error
	if errors.As(err, &ce) {
		c.sendErr(clientMsgID, ce.Code, ce.Code)
		return
	}
	c.sendErr(clientMsgID, "internal", "internal error")
}

func (c *conn) handle(f Frame) {
	h := c.hub
	ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancel()
	if f.ConversationID == nil {
		c.sendErr(f.ClientMsgID, "bad_frame", "conversation_id required")
		return
	}
	conv := *f.ConversationID
	switch f.Type {
	case "send":
		if f.ClientMsgID == nil {
			c.sendErr(nil, "bad_frame", "client_msg_id required")
			return
		}
		// Evaluate both keys so each bucket is charged.
		okU := h.msgLimiter.Allow("msg:u:" + c.userID.String())
		okIP := h.msgLimiter.Allow("msg:ip:" + c.ip)
		if !okU || !okIP {
			c.sendErr(f.ClientMsgID, chat.ErrRateLimited.Code, "too many messages, slow down")
			return
		}
		m, peer, dup, err := h.svc.Send(ctx, c.user, conv, *f.ClientMsgID, f.Body)
		if err != nil {
			c.svcErr(f.ClientMsgID, err)
			return
		}
		c.enqueue(Frame{Type: "ack", ClientMsgID: f.ClientMsgID, Message: &m})
		if !dup {
			h.deliver(peer, Frame{Type: "message", Message: &m})
		}
	case "typing":
		peer, deliver, err := h.svc.PeerOf(ctx, c.user, conv)
		if err != nil {
			c.svcErr(nil, err)
			return
		}
		if deliver {
			h.deliver(peer, Frame{Type: "typing", ConversationID: &conv, UserID: &c.userID})
		}
	case "read":
		seq, peer, notify, err := h.svc.MarkRead(ctx, c.user, conv, f.Seq)
		if err != nil {
			c.svcErr(nil, err)
			return
		}
		if notify {
			h.deliver(peer, Frame{Type: "read", ConversationID: &conv, UserID: &c.userID, Seq: seq})
		}
	default:
		c.sendErr(f.ClientMsgID, "bad_frame", "unknown frame type")
	}
}
