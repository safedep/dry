package aiservices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuardedPrompt(t *testing.T) {
	req := LLMGenerationRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Tell me a joke.",
	}

	r, err := guardedPrompt(req)
	assert.NoError(t, err)

	t.Run("system prompt should be hardened", func(t *testing.T) {
		assert.Contains(t, r.systemPrompt, req.SystemPrompt)
		assert.Contains(t, r.systemPrompt, "SECURITY RULES:")
	})

	t.Run("user prompt should be unchanged", func(t *testing.T) {
		assert.Equal(t, req.UserPrompt, r.userPrompt)
	})
}
