package match

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// testIdentity is a small identity for the tests of this package. A NameKey
// is the type and the escaped name. A version is dotted numbers. The real
// identity on the rules of dry/api/pb has its own tests in policy/pbidentity.
type testIdentity struct{}

type testPackage struct {
	key     string
	version string
}

func (testIdentity) Package(purl string) (Package, error) {
	typ, path, err := splitTestPurl(purl)
	if err != nil {
		return nil, err
	}

	if strings.Contains(path, "*") {
		return nil, errors.New("a package has no '*'")
	}

	name, version := path, ""
	if i := strings.LastIndex(path, "@"); i > 0 && path[i-1] != '/' {
		name, version = path[:i], path[i+1:]
	}

	if name == "" {
		return nil, errors.New("no name")
	}

	return testPackage{key: typ + "/" + url.QueryEscape(name), version: version}, nil
}

func (testIdentity) Prefix(purl string) (string, error) {
	typ, path, err := splitTestPurl(purl)
	if err != nil {
		return "", err
	}

	stem, ok := strings.CutSuffix(path, "*")
	if !ok || (stem != "" && !strings.HasSuffix(stem, "/") && !strings.HasSuffix(stem, ".")) {
		return "", errors.New("not a prefix")
	}

	return typ + "/" + url.QueryEscape(stem), nil
}

func splitTestPurl(purl string) (string, string, error) {
	rest, ok := strings.CutPrefix(purl, "pkg:")
	if !ok {
		return "", "", errors.New("no pkg scheme")
	}

	typ, path, ok := strings.Cut(rest, "/")
	if !ok || typ == "" || typ == "unknown" {
		return "", "", fmt.Errorf("bad type in %q", purl)
	}

	return typ, path, nil
}

func (p testPackage) NameKey() string { return p.key }
func (p testPackage) Version() string { return p.version }

func (p testPackage) CompareVersion(v string) (int, error) {
	a, err := parseTestVersion(p.version)
	if err != nil {
		return 0, err
	}

	b, err := parseTestVersion(v)
	if err != nil {
		return 0, err
	}

	for i := 0; i < max(len(a), len(b)); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}

		if x != y {
			if x < y {
				return -1, nil
			}
			return 1, nil
		}
	}

	return 0, nil
}

func parseTestVersion(v string) ([]int, error) {
	if v == "" {
		return nil, errors.New("empty version")
	}

	var parts []int
	for s := range strings.SplitSeq(v, ".") {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("version %q: %w", v, err)
		}
		parts = append(parts, n)
	}

	return parts, nil
}

func mustPackage(purl string) Package {
	p, err := testIdentity{}.Package(purl)
	if err != nil {
		panic(err)
	}
	return p
}
