package pb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalContainerImage(t *testing.T) {
	upper := "sha256:" + strings.Repeat("AB", 32)
	lower := "sha256:" + strings.Repeat("ab", 32)

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"hub official short name", "ubuntu", "docker.io/library/ubuntu:latest", false},
		{"explicit latest", "ubuntu:latest", "docker.io/library/ubuntu:latest", false},
		{"hub official with tag", "ubuntu:22.04", "docker.io/library/ubuntu:22.04", false},
		{"hub user repository", "evil/miner", "docker.io/evil/miner:latest", false},
		{"hub qualified", "docker.io/evil/miner", "docker.io/evil/miner:latest", false},
		{"hub index alias", "index.docker.io/library/nginx:1.25", "docker.io/library/nginx:1.25", false},
		{"hub registry-1 alias", "registry-1.docker.io/library/ubuntu", "docker.io/library/ubuntu:latest", false},
		{"docker scheme", "docker://evil/miner:v1", "docker.io/evil/miner:v1", false},
		{"surrounding space", "  evil/miner  ", "docker.io/evil/miner:latest", false},
		{"name lowercased, tag kept", "Evil/Miner:Latest", "docker.io/evil/miner:Latest", false},
		{"ghcr", "ghcr.io/evil-org/miner:v1", "ghcr.io/evil-org/miner:v1", false},
		{"ghcr uppercase", "GHCR.IO/Evil-Org/Miner", "ghcr.io/evil-org/miner:latest", false},
		{"ghcr nested path", "ghcr.io/evil-org/team/miner", "ghcr.io/evil-org/team/miner:latest", false},
		{"quay", "quay.io/evil/miner", "quay.io/evil/miner:latest", false},
		{"ecr", "123456789012.dkr.ecr.us-east-1.amazonaws.com/miner:prod", "123456789012.dkr.ecr.us-east-1.amazonaws.com/miner:prod", false},
		{"registry port", "Registry.Example.com:5000/a/b/c:dev", "registry.example.com:5000/a/b/c:dev", false},
		{"localhost", "localhost/evil", "localhost/evil:latest", false},
		{"localhost port", "localhost:5000/miner", "localhost:5000/miner:latest", false},
		{"ipv6 registry", "[::1]:5000/miner:v1", "[::1]:5000/miner:v1", false},
		{"ipv6 registry uppercase", "[FE80::1]/Miner", "[fe80::1]/miner:latest", false},
		{"digest without tag stays untagged", "ghcr.io/evil/img@" + upper, "ghcr.io/evil/img@" + lower, false},
		{"tag and digest", "evil/img:v1@" + lower, "docker.io/evil/img:v1@" + lower, false},

		{"empty", "", "", true},
		{"short digest", "evil/img@sha256:abc", "", true},
		{"other digest algorithm", "evil/img@sha512:" + strings.Repeat("ab", 64), "", true},
		{"tag starts with dash", "evil/img:-bad", "", true},
		{"tag with space", "evil/miner:bad tag", "", true},
		{"empty path component", "evil//img", "", true},
		{"url", "https://ghcr.io/evil/miner", "", true},
		{"bad ipv6 registry", "[zz::1]:5000/miner", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CanonicalContainerImage(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCanonicalContainerImageIsIdempotent(t *testing.T) {
	for _, in := range []string{"ubuntu", "Evil/Miner:V1", "ghcr.io/a/b@sha256:" + strings.Repeat("ab", 32), "localhost:5000/x:y"} {
		once, err := CanonicalContainerImage(in)
		require.NoError(t, err)
		twice, err := CanonicalContainerImage(once)
		require.NoError(t, err)
		assert.Equal(t, once, twice, in)
	}
}
