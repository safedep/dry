package match

// Identity turns a PURL into a canonical package identity. The tool supplies
// it, so the rules of the canonical package version identity stay in one
// place in each tool.
type Identity interface {
	// Package parses the PURL of one package, with or without a version.
	Package(purl string) (Package, error)

	// Namespace parses a PURL that ends with "/*" or ".*" and returns a
	// NameKey prefix. The NameKey of each package under the prefix starts
	// with the result, and the NameKey of a package outside it does not. The
	// prefix ends with the canonical form of its boundary, so
	// "pkg:npm/@acme/*" does not select "@acmex/x".
	Namespace(purl string) (string, error)
}

// Package is one package, with or without a version, in canonical form.
type Package interface {
	// NameKey is the canonical ecosystem and name.
	NameKey() string

	// Version is the canonical version, or "" when the package has none.
	Version() string

	// CompareVersion orders the version of the package against v, under the
	// order of the ecosystem. It returns -1, 0 or +1.
	CompareVersion(v string) (int, error)
}
