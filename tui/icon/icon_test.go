// tui/icon/icon_test.go
package icon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/safedep/dry/tui/output"
)

func TestIconResolve(t *testing.T) {
	i := Icon{Unicode: "✓", Ascii: "[OK]", Agent: "OK:"}
	assert.Equal(t, "✓", i.Resolve(output.Rich))
	assert.Equal(t, "[OK]", i.Resolve(output.Plain))
	assert.Equal(t, "OK:", i.Resolve(output.Agent))
}

func TestDefaultSetHasAllKeys(t *testing.T) {
	s := DefaultSet()
	for _, k := range AllKeys() {
		if k == KeySpinnerFrames {
			continue // special-cased; frames live in spinner pkg
		}
		i, ok := s.Get(k)
		assert.True(t, ok, "missing icon for key %v", k)
		assert.NotEmpty(t, i.Unicode, "key %v: empty Unicode", k)
		assert.NotEmpty(t, i.Ascii, "key %v: empty Ascii", k)
		assert.NotEmpty(t, i.Agent, "key %v: empty Agent", k)
	}
}
