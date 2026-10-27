package pb

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// The patterns follow the reference grammar of distribution/reference v0.6.0
// (regexp.go) and the registered digest algorithms of the OCI image spec.
var (
	containerRegistryPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+)?$`)
	containerIPv6RegistryRE  = regexp.MustCompile(`^\[([0-9a-f:]+)\](:[0-9]+)?$`)
	containerPathPattern     = regexp.MustCompile(`^[a-z0-9]+((?:[._]|__|-+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|-+)[a-z0-9]+)*)*$`)
	containerTagPattern      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)
	containerDigestPattern   = regexp.MustCompile(`^(sha256:[0-9a-f]{64}|sha512:[0-9a-f]{128})$`)
	containerImageIDPattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// containerRepositoryMaxLen is RepositoryNameTotalLengthMax in
// distribution/reference. It counts the path without the registry.
const containerRepositoryMaxLen = 255

// CanonicalContainerImage returns an image reference in the form
// registry/repository[:tag] or registry/repository@<digest>. A short Docker
// Hub name is fully qualified, as ParseNormalizedNamed in
// distribution/reference does: ubuntu is docker.io/library/ubuntu and
// evil/miner is docker.io/evil/miner. So two spellings of one image give one
// value, and a store can match on it.
//
// A reference with no tag stays untagged. It names the whole repository, so a
// consumer can match every tag by prefix. Adding latest is a pull-time choice
// (TagNameOnly), and it would narrow the reference to one moving tag.
//
// A reference with a digest drops its tag, as ParseDockerRef does. The
// registry serves the digest and ignores the tag, and runtimes record the
// image as repository@digest.
//
// The registry, the repository and the digest are lowercased. The tag keeps its
// case, because registries treat v1 and V1 as two tags.
func CanonicalContainerImage(ref string) (string, error) {
	in := ref
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "docker://")

	// Lowercasing would fold a non-ASCII name into an ASCII one that no
	// registry serves, so reject it first.
	for i := 0; i < len(ref); i++ {
		if ref[i] >= 0x80 {
			return "", fmt.Errorf("invalid container image %q: not ASCII", in)
		}
	}

	// A bare 64-hex string is an image ID, not a name. Without this check it
	// would become docker.io/library/<id>, an image that does not exist.
	if containerImageIDPattern.MatchString(strings.ToLower(ref)) {
		return "", fmt.Errorf("invalid container image %q: an image id is not a reference", in)
	}

	var digest string
	if i := strings.IndexByte(ref, '@'); i >= 0 {
		ref, digest = ref[:i], strings.ToLower(ref[i+1:])
		if !containerDigestPattern.MatchString(digest) {
			return "", fmt.Errorf("invalid container image digest %q: want sha256:<64 hex> or sha512:<128 hex>", digest)
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
	if !validContainerRegistry(registry) || !containerPathPattern.MatchString(repo) {
		return "", fmt.Errorf("invalid container image %q: want [registry/]repository[:tag][@digest]", in)
	}
	if len(repo) > containerRepositoryMaxLen {
		return "", fmt.Errorf("invalid container image %q: repository longer than %d characters", in, containerRepositoryMaxLen)
	}

	out := registry + "/" + repo
	if digest != "" {
		return out + "@" + digest, nil
	}
	if tag != "" {
		out += ":" + tag
	}
	return out, nil
}

// validContainerRegistry accepts a host name, an IPv4 address or a bracketed
// IPv6 address, each with an optional port. The grammar allows only hex digits
// and colons in brackets, so an IPv4 or IPv4-mapped address there is rejected.
func validContainerRegistry(registry string) bool {
	if m := containerIPv6RegistryRE.FindStringSubmatch(registry); m != nil {
		return net.ParseIP(m[1]) != nil
	}
	return containerRegistryPattern.MatchString(registry)
}
