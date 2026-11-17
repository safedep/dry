package apiguard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCustomKeySessionState(t *testing.T) {
	expiresAt := time.Unix(1_900_000_000, 0)

	cases := []struct {
		name                string
		args                KeyArgs
		wantMeta            map[string]any
		wantSessionLifetime int64
	}{
		{
			name: "tenant API key",
			args: KeyArgs{
				Info:      KeyInfo{OrganizationID: "org", TeamID: "team", UserID: "user", KeyID: "key"},
				ExpiresAt: expiresAt,
			},
			wantMeta: map[string]any{"org_id": "org", "team_id": "team", "user_id": "user", "key_id": "key"},
		},
		{
			name: "workload credential",
			args: KeyArgs{
				Info: KeyInfo{
					OrganizationID: "org",
					TeamID:         "team",
					KeyID:          "exchange",
					KeyType:        KeyTypeWorkload,
					Binding:        "binding",
				},
				ExpiresAt:       expiresAt,
				SessionLifetime: 6 * time.Hour,
			},
			wantMeta: map[string]any{
				"org_id":   "org",
				"team_id":  "team",
				"key_id":   "exchange",
				"key_type": KeyTypeWorkload,
				"binding":  "binding",
			},
			wantSessionLifetime: 21600,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				Expires         int64          `json:"expires"`
				SessionLifetime int64          `json:"session_lifetime"`
				MetaData        map[string]any `json:"meta_data"`
			}
			tyk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/tyk/keys/sfd_custom", r.URL.Path)
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"key":"sfd_custom","key_hash":"hash","status":"ok"}`))
				assert.NoError(t, err)
			}))
			t.Cleanup(tyk.Close)

			client, err := NewManagementClient(tyk.URL, "admin-secret")
			require.NoError(t, err)

			key, err := client.CreateCustomKey(context.Background(), "sfd_custom", tc.args)
			require.NoError(t, err)

			assert.Equal(t, "hash", key.KeyId)
			assert.Equal(t, expiresAt.Unix(), got.Expires)
			assert.Equal(t, tc.wantSessionLifetime, got.SessionLifetime)
			assert.Equal(t, tc.wantMeta, got.MetaData)
		})
	}
}

func TestKeyInfoJSONLeavesOutBinding(t *testing.T) {
	out, err := json.Marshal(KeyInfo{KeyID: "exchange", KeyType: KeyTypeWorkload, Binding: "binding-secret"})
	require.NoError(t, err)

	assert.NotContains(t, string(out), "binding-secret")
	assert.Contains(t, string(out), `"key_type":"workload"`)
}
