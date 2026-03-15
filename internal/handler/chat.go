package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gr8cally/ai-chatbot/internal/agent"
	"github.com/gr8cally/ai-chatbot/internal/session"
	"github.com/gr8cally/ai-chatbot/templates"
)

type pendingStream struct {
	Messages []agent.RequestMessage
	Cancel   context.CancelFunc
}

type ChatHandler struct {
	sessions       *session.Store
	agent          *agent.Client
	pendingStreams sync.Map
	activeStreams  sync.Map
}

func NewChatHandler(sessions *session.Store, agentClient *agent.Client) *ChatHandler {
	return &ChatHandler{
		sessions: sessions,
		agent:    agentClient,
	}
}

const maxMessageLength = 4000

func (h *ChatHandler) HandleSend(w http.ResponseWriter, r *http.Request) {
	sessionID := getOrCreateSession(w, r)
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}
	if len(message) > maxMessageLength {
		templates.ErrorBubble("Message is too long. Please shorten it and try again.").Render(r.Context(), w)
		return
	}

	userMsg := session.Message{
		ID:        uuid.New().String(),
		Role:      session.RoleUser,
		Content:   message,
		Timestamp: time.Now(),
	}
	h.sessions.AddMessage(sessionID, userMsg)

	reqMessages := buildRequestMessages(h.sessions.GetMessages(sessionID))

	content, err := h.agent.Invoke(r.Context(), reqMessages)
	if err != nil {
		log.Printf("Agent invoke error: %v", err)
		templates.ErrorBubble("Something went wrong. Please try again.").Render(r.Context(), w)
		return
	}

	assistantMsg := session.Message{
		ID:        uuid.New().String(),
		Role:      session.RoleAssistant,
		Content:   content,
		Timestamp: time.Now(),
	}
	h.sessions.AddMessage(sessionID, assistantMsg)

	templates.MessagePair(userMsg, assistantMsg).Render(r.Context(), w)
}

func (h *ChatHandler) HandleStreamStart(w http.ResponseWriter, r *http.Request) {
	sessionID := getOrCreateSession(w, r)
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}
	if len(message) > maxMessageLength {
		templates.ErrorBubble("Message is too long. Please shorten it and try again.").Render(r.Context(), w)
		return
	}

	userMsg := session.Message{
		ID:        uuid.New().String(),
		Role:      session.RoleUser,
		Content:   message,
		Timestamp: time.Now(),
	}
	h.sessions.AddMessage(sessionID, userMsg)

	streamID := uuid.New().String()
	reqMessages := buildRequestMessages(h.sessions.GetMessages(sessionID))
	h.pendingStreams.Store(streamID, &pendingStream{
		Messages: reqMessages,
	})

	templates.StreamStart(userMsg, streamID).Render(r.Context(), w)
}

func (h *ChatHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	sessionID := getOrCreateSession(w, r)
	streamID := r.URL.Query().Get("stream_id")
	if streamID == "" {
		http.Error(w, "Missing stream_id", http.StatusBadRequest)
		return
	}

	val, ok := h.pendingStreams.LoadAndDelete(streamID)
	if !ok {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}
	pending := val.(*pendingStream)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	h.activeStreams.Store(sessionID, cancel)
	defer h.activeStreams.Delete(sessionID)

	events, err := h.agent.Stream(ctx, pending.Messages)
	if err != nil {
		log.Printf("Agent stream error: %v", err)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", "Failed to connect to agent")
		flusher.Flush()
		return
	}

	var fullText strings.Builder
	for event := range events {
		select {
		case <-ctx.Done():
			return
		default:
		}

		switch event.Type {
		case "content_block_delta":
			fullText.WriteString(event.Text)
			escaped := strings.ReplaceAll(event.Text, "\n", "\\n")
			escaped = strings.ReplaceAll(escaped, "\r", "\\r")
			fmt.Fprintf(w, "event: delta\ndata: %s\n\n", escaped)
			flusher.Flush()
		case "message_stop":
			if event.Role == "assistant" {
				fmt.Fprintf(w, "event: done\ndata: complete\n\n")
				flusher.Flush()
			}
		}
	}

	// Always send a done event if we exit the loop without one (as a safety)
	fmt.Fprintf(w, "event: done\ndata: complete\n\n")
	flusher.Flush()

	if fullText.Len() > 0 {
		h.sessions.AddMessage(sessionID, session.Message{
			ID:        uuid.New().String(),
			Role:      session.RoleAssistant,
			Content:   fullText.String(),
			Timestamp: time.Now(),
		})
	}
}

func (h *ChatHandler) HandleAbort(w http.ResponseWriter, r *http.Request) {
	sessionID := getOrCreateSession(w, r)
	if val, ok := h.activeStreams.Load(sessionID); ok {
		cancel := val.(context.CancelFunc)
		cancel()
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ChatHandler) HandleClear(w http.ResponseWriter, r *http.Request) {
	sessionID := getOrCreateSession(w, r)
	h.sessions.Clear(sessionID)
	templates.EmptyChat().Render(r.Context(), w)
}

func buildRequestMessages(messages []session.Message) []agent.RequestMessage {
	reqMsgs := make([]agent.RequestMessage, len(messages))
	for i, msg := range messages {
		role := "user"
		if msg.Role == session.RoleAssistant {
			role = "assistant"
		}
		reqMsgs[i] = agent.RequestMessage{
			Role:    role,
			Content: msg.Content,
		}
	}
	return reqMsgs
}

func getOrCreateSession(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	sessionID := uuid.New().String()
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7, // 7 days
	})
	return sessionID
}
