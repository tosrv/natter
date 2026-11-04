// Package api wires the REST routes.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tosrv/natter/internal/auth"
)

// WriteError maps *chat.Error to its status and the standard error JSON;
// anything else becomes 500 "internal" (detail only in the gin error log).
func WriteError(c *gin.Context, err error) { auth.WriteError(c, err) }
