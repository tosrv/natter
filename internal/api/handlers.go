package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/ratelimit"
	"github.com/tosrv/natter/internal/store"
)

const maxUsernameChanges = 3

var errBadRequest = &chat.Error{Code: "bad_request", Status: http.StatusBadRequest}

type handlers struct {
	chat     *chat.Service
	limiter  *ratelimit.Limiter
	auth     *auth.Auth
	notifier Notifier
}

func meJSON(u store.User) gin.H {
	left := maxUsernameChanges - u.UsernameChanges
	if left < 0 {
		left = 0
	}
	h := gin.H{"id": u.ID, "kind": u.Kind, "username": deref(u.Username), "display_name": u.DisplayName,
		"is_developer": u.IsDeveloper, "username_changes_left": left}
	if u.Email != nil {
		h["email"] = *u.Email
	}
	return h
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (h *handlers) me(c *gin.Context) { c.JSON(http.StatusOK, meJSON(auth.CurrentUser(c))) }

func (h *handlers) setUsername(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		WriteError(c, errBadRequest)
		return
	}
	u, err := h.chat.SetUsername(c.Request.Context(), auth.CurrentUser(c), in.Username)
	if err != nil {
		WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, meJSON(u))
}

func (h *handlers) setDisplayName(c *gin.Context) {
	var in struct {
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		WriteError(c, errBadRequest)
		return
	}
	u, err := h.chat.SetDisplayName(c.Request.Context(), auth.CurrentUser(c), in.DisplayName)
	if err != nil {
		WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, meJSON(u))
}

func (h *handlers) deleteMe(c *gin.Context) {
	u := auth.CurrentUser(c)
	if u.Kind == "guest" {
		h.endGuestAndClear(c, u.ID)
		return
	}
	peers, err := h.chat.DeleteAccount(c.Request.Context(), u)
	if err != nil {
		WriteError(c, err)
		return
	}
	h.auth.ClearSession(c)
	if h.notifier != nil {
		h.notifier.NotifyConversationDeleted(peers, u.ID)
	}
	c.Status(http.StatusNoContent)
}

// endGuestAndClear ends the guest and clears its session. A guest already ended
// concurrently (ErrNotFound) still gets the cookie cleared and a 204, without
// notifying anyone.
func (h *handlers) endGuestAndClear(c *gin.Context, guestID uuid.UUID) {
	out, convs, err := h.chat.EndGuest(c.Request.Context(), guestID)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, chat.ErrNotFound) {
		WriteError(c, err)
		return
	}
	h.auth.ClearSession(c)
	if err == nil && h.notifier != nil {
		h.notifier.NotifyGuestEnded(out, convs, guestID)
	}
	c.Status(http.StatusNoContent)
}

func (h *handlers) search(c *gin.Context) {
	u := auth.CurrentUser(c)
	if h.limiter != nil && !h.limiter.Allow("search:"+u.ID.String()) {
		WriteError(c, chat.ErrRateLimited)
		return
	}
	res, err := h.chat.Search(c.Request.Context(), u, c.Query("q"))
	if err != nil {
		WriteError(c, err)
		return
	}
	out := make([]gin.H, 0, len(res))
	for _, b := range res {
		out = append(out, gin.H{"id": b.ID, "username": b.Username, "display_name": b.DisplayName})
	}
	c.JSON(http.StatusOK, out)
}

func (h *handlers) openConversation(c *gin.Context) {
	var in struct {
		PeerID string `json:"peer_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		WriteError(c, errBadRequest)
		return
	}
	pid, err := uuid.Parse(in.PeerID)
	if err != nil {
		WriteError(c, errBadRequest)
		return
	}
	id, err := h.chat.OpenConversation(c.Request.Context(), auth.CurrentUser(c), pid)
	if err != nil {
		WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

func (h *handlers) conversations(c *gin.Context) {
	cs, err := h.chat.Conversations(c.Request.Context(), auth.CurrentUser(c))
	if err != nil {
		WriteError(c, err)
		return
	}
	if cs == nil {
		cs = []store.ConversationSummary{}
	}
	c.JSON(http.StatusOK, cs)
}

func intQuery(c *gin.Context, key string, def int64) (int64, error) {
	s := c.Query(key)
	if s == "" {
		return def, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

func (h *handlers) messages(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		WriteError(c, chat.ErrNotFound)
		return
	}
	after, e1 := intQuery(c, "after_seq", 0)
	before, e2 := intQuery(c, "before_seq", 0)
	limit, e3 := intQuery(c, "limit", 50)
	if e1 != nil || e2 != nil || e3 != nil {
		WriteError(c, errBadRequest)
		return
	}
	if limit > 100 {
		limit = 100
	}
	ms, err := h.chat.Messages(c.Request.Context(), auth.CurrentUser(c), cid, after, before, int(limit))
	if err != nil {
		WriteError(c, err)
		return
	}
	if ms == nil {
		ms = []store.Message{}
	}
	c.JSON(http.StatusOK, gin.H{"messages": ms})
}

func (h *handlers) clearConversation(c *gin.Context) {
	cid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		WriteError(c, chat.ErrNotFound)
		return
	}
	u := auth.CurrentUser(c)
	if err := h.chat.ClearConversation(c.Request.Context(), u, cid); err != nil {
		WriteError(c, err)
		return
	}
	if h.notifier != nil {
		h.notifier.NotifyConversationCleared(u.ID, cid)
	}
	c.Status(http.StatusNoContent)
}
