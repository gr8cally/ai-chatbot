package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	ID        string
	Role      Role
	Content   string
	Timestamp time.Time
}

type Session struct {
	ID        string
	Messages  []Message
	CreatedAt time.Time
	mu        sync.RWMutex
}

type Store struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

func (s *Store) GetOrCreate(sessionID string) *Session {
	s.mu.RLock()
	sess, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if ok {
		return sess
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if sess, ok := s.sessions[sessionID]; ok {
		return sess
	}

	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	sess = &Session{
		ID:        sessionID,
		Messages:  make([]Message, 0),
		CreatedAt: time.Now(),
	}
	s.sessions[sessionID] = sess
	return sess
}

func (s *Store) AddMessage(sessionID string, msg Message) {
	sess := s.GetOrCreate(sessionID)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	sess.Messages = append(sess.Messages, msg)
}

func (s *Store) GetMessages(sessionID string) []Message {
	sess := s.GetOrCreate(sessionID)
	sess.mu.RLock()
	defer sess.mu.RUnlock()

	msgs := make([]Message, len(sess.Messages))
	copy(msgs, sess.Messages)
	return msgs
}

func (s *Store) Clear(sessionID string) {
	sess := s.GetOrCreate(sessionID)
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.Messages = make([]Message, 0)
}
