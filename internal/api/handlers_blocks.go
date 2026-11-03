package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/store"
)

func (h *handlers) block(c *gin.Context) {
	var in struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		WriteError(c, errBadRequest)
		return
	}
	id, err := uuid.Parse(in.UserID)
	if err != nil {
		WriteError(c, errBadRequest)
		return
	}
	if err := h.chat.Block(c.Request.Context(), auth.CurrentUser(c), id); err != nil {
		WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handlers) unblock(c *gin.Context) {
	id, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		WriteError(c, chat.ErrNotFound)
		return
	}
	if err := h.chat.Unblock(c.Request.Context(), auth.CurrentUser(c), id); err != nil {
		WriteError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *handlers) blocks(c *gin.Context) {
	bs, err := h.chat.Blocks(c.Request.Context(), auth.CurrentUser(c))
	if err != nil {
		WriteError(c, err)
		return
	}
	if bs == nil {
		bs = []store.BlockedUser{}
	}
	c.JSON(http.StatusOK, bs)
}
