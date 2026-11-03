package configsync

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A long error text must not make a state.json that State refuses, or the
// tool loses the policy that is on disk.
func TestFetch_LongErrorTextKeepsTheStateReadable(t *testing.T) {
	long := strings.Repeat("é", maxStateSize)

	cases := []struct {
		name         string
		arrange      func(f *fixture, req *FetchRequest)
		wantRejected bool
	}{
		{
			name: "grpc error with a long message",
			arrange: func(f *fixture, _ *FetchRequest) {
				f.reply(nil, status.Error(codes.Unavailable, long))
			},
		},
		{
			name: "validator error with a long text",
			arrange: func(f *fixture, req *FetchRequest) {
				f.reply(modified(policyB, 2), nil)
				req.Validate = refuseWith(errors.New(long))
			},
			wantRejected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(tenantA, policyA, 1)

			f.tick()
			req := request(tenantA)
			tc.arrange(f, &req)
			_, err := f.fetcher.Fetch(context.Background(), req)
			require.Error(t, err)

			st, err := f.store.State()
			require.NoError(t, err, "the state must stay readable")
			assert.LessOrEqual(t, len(st.LastError), maxTextSize+len(" [truncated]"))
			assert.True(t, strings.HasSuffix(st.LastError, " [truncated]"))
			if tc.wantRejected {
				require.NotNil(t, st.Rejected)
				assert.LessOrEqual(t, len(st.Rejected.Reason), maxTextSize+len(" [truncated]"))
			}

			loaded, err := f.store.Load(acceptAll)
			require.NoError(t, err)
			assert.Equal(t, policyA, loaded.Policy)
		})
	}
}

func TestStore_WriteStateRefusesAnOversizedState(t *testing.T) {
	f := newFixture(t)
	f.commit(tenantA, policyA, 1)
	before := f.state()

	big := f.state()
	big.LastError = strings.Repeat("x", maxStateSize)
	err := f.store.writeState(big)

	require.ErrorIs(t, err, ErrTooLarge)
	assert.Equal(t, before, f.state(), "a refused write must keep the old state")
}

func TestClipText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "short text stays", in: "unavailable", want: "unavailable"},
		{name: "text at the limit stays", in: strings.Repeat("a", maxTextSize), want: strings.Repeat("a", maxTextSize)},
		{name: "long text is cut", in: strings.Repeat("a", maxTextSize+1), want: strings.Repeat("a", maxTextSize) + " [truncated]"},
		{
			name: "cut does not split a rune",
			in:   strings.Repeat("a", maxTextSize-1) + "é",
			want: strings.Repeat("a", maxTextSize-1) + " [truncated]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, clipText(tc.in))
		})
	}
}
