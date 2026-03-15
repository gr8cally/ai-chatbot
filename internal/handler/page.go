package handler

import (
	"net/http"

	"github.com/gr8cally/ai-chatbot/internal/session"
	"github.com/gr8cally/ai-chatbot/templates"
)

type PageHandler struct {
	sessions *session.Store
}

func NewPageHandler(sessions *session.Store) *PageHandler {
	return &PageHandler{sessions: sessions}
}

func (h *PageHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	sessionID := getOrCreateSession(w, r)
	messages := h.sessions.GetMessages(sessionID)
	templates.ChatPage(messages).Render(r.Context(), w)
}
