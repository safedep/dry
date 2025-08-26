package agent

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// Memory is the interface for implementing memory for agents.
type Memory interface {
	// AddInteraction adds an interaction to the memory.
	AddInteraction(ctx context.Context, interaction *schema.Message) error

	// GetInteractions gets the interactions from the memory.
	GetInteractions(ctx context.Context) ([]*schema.Message, error)

	// Clear clears the memory.
	Clear(ctx context.Context) error
}

type simpleMemory struct {
	mutex        sync.RWMutex
	interactions []*schema.Message
}

var _ Memory = (*simpleMemory)(nil)

// NewSimpleMemory creates a new simple memory. This memory is dumb and only
// stores interactions in memory. It is not persisted and will be lost when the
// agent is stopped.
func NewSimpleMemory() (*simpleMemory, error) {
	return &simpleMemory{}, nil
}

func (m *simpleMemory) AddInteraction(ctx context.Context, interaction *schema.Message) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.interactions = append(m.interactions, interaction)

	return nil
}

func (m *simpleMemory) GetInteractions(ctx context.Context) ([]*schema.Message, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.interactions, nil
}

func (m *simpleMemory) Clear(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.interactions = make([]*schema.Message, 0)
	return nil
}
