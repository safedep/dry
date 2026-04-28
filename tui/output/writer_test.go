// tui/output/writer_test.go
package output

import (
	"bytes"
	"errors"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriterDefaults(t *testing.T) {
	ResetWritersForTest()
	assert.NotNil(t, Stdout())
	assert.NotNil(t, Stderr())
}

func TestSetWriters(t *testing.T) {
	var out, err bytes.Buffer
	SetWriters(&out, &err)
	defer ResetWritersForTest()

	_, _ = Stdout().Write([]byte("data"))
	_, _ = Stderr().Write([]byte("chatter"))

	assert.Equal(t, "data", out.String())
	assert.Equal(t, "chatter", err.String())
}

func TestWriterSerializesConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	SetWriters(&buf, &buf)
	defer ResetWritersForTest()

	const N = 100
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = Stderr().Write([]byte("ABCDEFGH\n"))
		}()
	}
	wg.Wait()

	// Every line should be intact — no interleaving mid-byte.
	out := buf.String()
	assert.Equal(t, N*9, len(out))
	for i := 0; i < N; i++ {
		line := out[i*9 : (i+1)*9]
		assert.Equal(t, "ABCDEFGH\n", line)
	}
}

// fakeEPIPEWriter returns syscall.EPIPE on Write.
type fakeEPIPEWriter struct{}

func (fakeEPIPEWriter) Write(p []byte) (int, error) { return 0, syscall.EPIPE }

func TestWriterSwallowsEPIPE(t *testing.T) {
	SetWriters(fakeEPIPEWriter{}, fakeEPIPEWriter{})
	defer ResetWritersForTest()

	n, err := Stdout().Write([]byte("x"))
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestWriterPropagatesOtherErrors(t *testing.T) {
	SetWriters(errWriter{errors.New("disk full")}, errWriter{errors.New("disk full")})
	defer ResetWritersForTest()

	_, err := Stdout().Write([]byte("x"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

type errWriter struct{ err error }

func (w errWriter) Write(p []byte) (int, error) { return 0, w.err }
