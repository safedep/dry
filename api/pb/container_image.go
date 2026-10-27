package pb

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	containerRegistryPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]{1,5})?$`)
	containerPathPattern     = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$`)
	containerTagPattern      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	containerDigestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// CanonicalContainerImage returns an image reference in the form
// registry/repository[:tag][@sha256:<digest>]. A short Docker Hub name is
// fully qualified, the way the Docker CLI resolves it: ubuntu is
// docker.io/library/ubuntu and evil/miner is docker.io/evil/miner. So two
// spellings of one image give one value, and a store can match on it.
//
// The registry, the repository and the digest are lowercased. The tag keeps its
// case, because registries treat v1 and V1 as two tags. The tag and the digest
// stay in the value: each one names a different image.
func CanonicalContainerImage(ref string) (string, error) {
	in := ref
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "docker://")

	var digest string
	if i := strings.IndexByte(ref, '@'); i >= 0 {
		ref, digest = ref[:i], strings.ToLower(ref[i+1:])
		if !containerDigestPattern.MatchString(digest) {
			return "", fmt.Errorf("invalid container image digest %q: want sha256:<64 hex>", digest)
		}
	}

	// A colon after the last slash starts the tag. One before it is a
	// registry port.
	var tag string
	if i := strings.LastIndexByte(ref, ':'); i > strings.LastIndexByte(ref, '/') {
		ref, tag = ref[:i], ref[i+1:]
		if !containerTagPattern.MatchString(tag) {
			return "", fmt.Errorf("invalid container image tag %q", tag)
		}
	}

	// The first component is a registry only when it has a dot or a port, or
	// is localhost. The Docker CLI uses the same test, so evil/miner is a
	// Docker Hub repository and not a registry named evil.
	registry, repo := "docker.io", strings.ToLower(ref)
	if first, rest, ok := strings.Cut(repo, "/"); ok &&
		(strings.ContainsAny(first, ".:") || first == "localhost") {
		registry, repo = first, rest
	}
	if registry == "index.docker.io" || registry == "registry-1.docker.io" {
		registry = "docker.io"
	}
	if registry == "docker.io" && !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	if !containerRegistryPattern.MatchString(registry) || !containerPathPattern.MatchString(repo) {
		return "", fmt.Errorf("invalid container image %q: want [registry/]repository[:tag][@sha256:digest]", in)
	}

	out := registry + "/" + repo
	if tag != "" {
		out += ":" + tag
	}
	if digest != "" {
		out += "@" + digest
	}
	return out, nil
}
