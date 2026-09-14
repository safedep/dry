package inbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedeliveryBackoff_EscalatesPerRecordAndCaps(t *testing.T) {
	b := redeliveryBackoff{base: 10 * time.Millisecond, max: 40 * time.Millisecond}

	assert.Equal(t, time.Duration(0), b.next("a"), "first redelivery is immediate")
	assert.Equal(t, 10*time.Millisecond, b.next("a"))
	assert.Equal(t, 20*time.Millisecond, b.next("a"))
	assert.Equal(t, 40*time.Millisecond, b.next("a"))
	assert.Equal(t, 40*time.Millisecond, b.next("a"), "capped at max")

	assert.Equal(t, time.Duration(0), b.next("b"), "a different record restarts the schedule")
	assert.Equal(t, 10*time.Millisecond, b.next("b"))

	b.reset()
	assert.Equal(t, time.Duration(0), b.next("b"), "reset clears the escalation")
}

func TestRedeliveryBackoff_ClampsWhenDoublingOvershootsMax(t *testing.T) {
	b := redeliveryBackoff{base: 30 * time.Millisecond, max: 40 * time.Millisecond}

	assert.Equal(t, time.Duration(0), b.next("a"))
	assert.Equal(t, 30*time.Millisecond, b.next("a"))
	assert.Equal(t, 40*time.Millisecond, b.next("a"), "clamps to max instead of doubling past it")
}
