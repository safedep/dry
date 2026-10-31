// Package pbidentity gives a match.Identity on the rules of the canonical
// package version identity in dry/api/pb.
//
// pmg and control-tower use it, so both compare packages by the same rules.
// A tool whose rules forbid an import of dry/api/pb, such as vet, supplies
// its own match.Identity instead.
package pbidentity

import (
	"sync"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"

	"github.com/safedep/dry/api/pb"
	"github.com/safedep/dry/policy/match"
)

// Identity implements match.Identity with pb.PackageVersion.
type Identity struct{}

var _ match.Identity = Identity{}

// Package parses the PURL with pb.NewPackageVersionFromPurl.
func (Identity) Package(purl string) (match.Package, error) {
	pv, err := pb.NewPackageVersionFromPurl(purl)
	if err != nil {
		return nil, err
	}

	return Package(pv), nil
}

// Prefix returns pb.NameKeyPrefix of the PURL.
func (Identity) Prefix(purl string) (string, error) {
	return pb.NameKeyPrefix(purl)
}

// Package returns the match.Package of a pb.PackageVersion. A tool that
// already holds a pb.PackageVersion for a request uses it to build a
// match.Query with no second parse.
func Package(pv pb.PackageVersion) match.Package {
	return &pbPackage{pv: pv, key: pv.NameKey()}
}

type pbPackage struct {
	pv  pb.PackageVersion
	key string

	// A parse of a version costs microseconds, and a range compares one
	// version with each of its bounds. So the package parses its own version
	// once, at the first compare.
	once    sync.Once
	version pb.OrderedVersion
	err     error
}

func (p *pbPackage) NameKey() string {
	return p.key
}

func (p *pbPackage) Version() string {
	return p.pv.Version()
}

// CompareVersion orders the raw version, as pb.PackageVersion.Compare does,
// so a version that the identity rule could not parse still gets the order
// of the ecosystem.
func (p *pbPackage) CompareVersion(v string) (int, error) {
	p.once.Do(func() {
		p.version, p.err = pb.ParseVersion(p.pv.Ecosystem(), p.pv.RawVersion())
	})
	if p.err != nil {
		return 0, p.err
	}

	other, err := boundVersions.parse(p.pv.Ecosystem(), v)
	if err != nil {
		return 0, err
	}

	return p.version.Compare(other)
}

// maxCachedVersions limits the cache. The versions come from the vers ranges
// of the rules, so a tenant with 10,000 rules fills a small part of it. Past
// the limit, a parse is not cached and the result stays the same.
const maxCachedVersions = 1 << 16

type parsedVersion struct {
	version pb.OrderedVersion
	err     error
}

// versionCache holds the parsed bound versions of vers ranges. A parse
// depends only on the ecosystem and the text, so one cache serves every
// index of the process.
type versionCache struct {
	mu       sync.RWMutex
	versions map[packagev1.Ecosystem]map[string]parsedVersion
	size     int
}

var boundVersions = &versionCache{versions: map[packagev1.Ecosystem]map[string]parsedVersion{}}

func (c *versionCache) parse(ecosystem packagev1.Ecosystem, v string) (pb.OrderedVersion, error) {
	c.mu.RLock()
	got, ok := c.versions[ecosystem][v]
	c.mu.RUnlock()

	if ok {
		return got.version, got.err
	}

	version, err := pb.ParseVersion(ecosystem, v)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.size < maxCachedVersions {
		byVersion := c.versions[ecosystem]
		if byVersion == nil {
			byVersion = map[string]parsedVersion{}
			c.versions[ecosystem] = byVersion
		}

		if _, ok := byVersion[v]; !ok {
			byVersion[v] = parsedVersion{version: version, err: err}
			c.size++
		}
	}

	return version, err
}
