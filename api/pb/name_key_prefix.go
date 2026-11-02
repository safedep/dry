package pb

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/package-url/packageurl-go"
)

// prefixVersionPattern finds a version in a prefix. An "@" that follows "/"
// starts an npm scope. Any other "@" starts a version. The policy contract
// uses the same rule (package_selector.prefix_has_no_version).
var prefixVersionPattern = regexp.MustCompile(`[^/]@`)

// NameKeyPrefix returns the NameKey prefix of a prefix PURL: a PURL that ends
// with "/*" or ".*", as in "pkg:npm/@acme/*" or "pkg:maven/org.apache.*". The
// NameKey of each package under the prefix starts with the result. The NameKey
// of a package outside it does not.
//
// The function reads the boundary from the raw text before it decodes it. So
// an encoded "/" or "." is not a boundary, and "pkg:npm/@acme/*" does not
// select "@acmex/x". It then folds the part before the "*" with the rule of
// the ecosystem, so "pkg:pypi/foo.*" also selects "foo-bar" and "foo_bar".
// Every name fold maps one character at a time, so the fold of the part is a
// prefix of the fold of each name that starts with the part.
//
// A prefix has no version, qualifiers or subpath, and only one "*". The
// result for "pkg:npm/*" selects every npm package.
func NameKeyPrefix(purl string) (string, error) {
	stem, ok := strings.CutSuffix(purl, "*")
	if !ok {
		return "", fmt.Errorf("prefix purl %q does not end with \"*\"", purl)
	}

	if !strings.HasSuffix(stem, "/") && !strings.HasSuffix(stem, ".") {
		return "", fmt.Errorf("prefix purl %q does not end with \"/*\" or \".*\"", purl)
	}

	if strings.ContainsAny(stem, "*?#") {
		return "", fmt.Errorf("prefix purl %q has a second \"*\", qualifiers or a subpath", purl)
	}

	if strings.Contains(strings.ToLower(stem), "%2a") {
		return "", fmt.Errorf("prefix purl %q has an encoded \"*\"", purl)
	}

	scheme, rest, ok := strings.Cut(stem, ":")
	if !ok || !strings.EqualFold(scheme, "pkg") {
		return "", fmt.Errorf("prefix purl %q does not start with \"pkg:\"", purl)
	}

	typ, path, ok := strings.Cut(strings.TrimLeft(rest, "/"), "/")
	if !ok {
		return "", fmt.Errorf("prefix purl %q has no type", purl)
	}

	if strings.HasPrefix(path, "/") || strings.Contains(path, "//") {
		return "", fmt.Errorf("prefix purl %q has an empty namespace segment", purl)
	}

	if prefixVersionPattern.MatchString(path) {
		return "", fmt.Errorf("prefix purl %q has a version", purl)
	}

	ecosystem := identityPurlEcosystem(strings.ToLower(typ))
	if ecosystem == packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED {
		return "", fmt.Errorf("unsupported purl type: %q", typ)
	}

	name, err := prefixIdentityName(ecosystem, path)
	if err != nil {
		return "", fmt.Errorf("prefix purl %q: %w", purl, err)
	}

	return NewPackageVersionFromParts(ecosystem, name, "").NameKey(), nil
}

// prefixIdentityName joins the namespace and the name part of a prefix the
// way purlIdentityName joins them for a package. The name part is empty when
// the prefix ends with "/".
func prefixIdentityName(ecosystem packagev1.Ecosystem, path string) (string, error) {
	var p packageurl.PackageURL

	rawName := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		namespace, err := purlNamespace(path[:i])
		if err != nil {
			return "", err
		}

		p.Namespace = namespace
		rawName = path[i+1:]
	}

	name, err := url.PathUnescape(rawName)
	if err != nil {
		return "", err
	}
	p.Name = name

	identityName := purlIdentityName(ecosystem, p)
	if p.Namespace != "" && identityName == p.Name {
		return "", errors.New("the ecosystem has no namespace")
	}

	return identityName, nil
}
