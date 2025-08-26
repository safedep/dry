package agent

import "github.com/google/uuid"

// Session is the interface for implementing sessions for agents.
type Session interface {
	// ID returns the session ID.
	ID() string

	// Memory returns the memory for the session.
	Memory() Memory
}

type session struct {
	sessionID string
	memory    Memory
}

var _ Session = &session{}

// NewSession creates a new session with a new memory.
// All session dependencies should be created before calling this function
// and passed to the session.
func NewSession(memory Memory) (*session, error) {
	return newSessionWithID(uuid.New().String(), memory), nil
}

func newSessionWithID(id string, memory Memory) *session {
	return &session{
		sessionID: id,
		memory:    memory,
	}
}

func (s *session) ID() string {
	return s.sessionID
}

func (s *session) Memory() Memory {
	return s.memory
}
