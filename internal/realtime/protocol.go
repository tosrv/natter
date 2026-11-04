// Package realtime implements the WebSocket hub (spec §5).
package realtime

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
)

// Frame is the single JSON envelope for every WS frame.
// For "error" frames the human-readable text is ErrMessage, serialized under
// the "message" key as a string (spec §5); otherwise "message" is Message.
type Frame struct {
	Type           string         `json:"type"`
	ConversationID *uuid.UUID     `json:"conversation_id,omitempty"`
	ClientMsgID    *uuid.UUID     `json:"client_msg_id,omitempty"`
	Body           string         `json:"body,omitempty"`
	Seq            int64          `json:"seq,omitempty"`
	UserID         *uuid.UUID     `json:"user_id,omitempty"`
	Message        *store.Message `json:"message,omitempty"`
	Code           string         `json:"code,omitempty"`
	ErrMessage     string         `json:"-"`
}

func (f Frame) MarshalJSON() ([]byte, error) {
	type plain Frame
	if f.Type != "error" {
		return json.Marshal(plain(f))
	}
	return json.Marshal(struct {
		plain
		Message string `json:"message,omitempty"`
	}{plain(f), f.ErrMessage})
}
