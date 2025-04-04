package packageregistry

import (
	"fmt"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// PublisherVerificationStatus holds package registry specific information about verification
// status of a publisher in the registry.
type PublisherVerificationStatus struct {
	IsVerified bool `json:"is_verified"`
}

// Publisher represents an individual or an organization having an account with
// a package registry for publishing package artifacts.
type Publisher struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Url   string `json:"url"`

	// Verification may or may not be supported by all
	// package registries. Hence its optional
	VerificationStatus *PublisherVerificationStatus `json:"verification_status"`
}

// PackagePublisherInfo represents the publisher of a package in a package registry.
type PackagePublisherInfo struct {
	Publishers []Publisher `json:"publishers"`
}

// PackageVersionInfo represents a version of a package in a package registry.
type PackageVersionInfo struct {
	// The version of the project.
	Version string `json:"version"`
}

// Package represents a package in a package registry.
// Example: `requests` in PyPI, `rails` in RubyGems.
type Package struct {
	// The name of the project.
	Name string `json:"name"`

	// The project description.
	Description string `json:"description"`

	// Normalized Git source URL for the project.
	// It will be in the format of https://github.com/owner/repo
	SourceRepositoryUrl string `json:"source_repository_url"`

	// Author is the creator of the package
	Author Publisher `json:"author"`

	// Maintainers of the package (can include author also)
	Maintainers []Publisher `json:"maintainers"`

	// Published versions of the project.
	Versions []PackageVersionInfo `json:"versions"`

	// Number of downloads of the package
	Downloads OptionalInt `json:"downloads"`

	// Package creation timestamps
	CreatedAt time.Time `json:"created_at"`

	// Package update timestamps
	UpdatedAt time.Time `json:"updated_at"`
}

// PackageDiscovery is a contract for implementing package discovery for a package registry.
// It exposes the package metadata for a given package name.
type PackageDiscovery interface {
	// GetPackage returns the package metadata for the given package name
	GetPackage(packageName string) (*Package, error)
}

// Contract for implementing publisher discovery for a package registry.
type PublisherDiscovery interface {
	// GetPackagePublisher returns the publishers for the given package.
	GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error)

	// Get packages published by a given publisher.
	GetPublisherPackages(publisher Publisher) ([]*Package, error)
}

// Client is a contract for implementing package registry
// clients for fetching various package and publisher metadata.
type Client interface {
	// Returns the publisher discovery client for the given package registry.
	// If the package registry does not support publisher discovery, it should
	// return an error.
	PublisherDiscovery() (PublisherDiscovery, error)

	// Returns the package discovery client for the given package registry.
	// If the package registry does not support package discovery, it should
	// return an error.
	PackageDiscovery() (PackageDiscovery, error)
}

// NewRegistryAdapter creates and returns a new registry adapter for the specified ecosystem.
//
// Parameters:
//   - ecosystem: The package ecosystem to create an adapter for (e.g., NPM, PyPI, RubyGems)
//
// Returns:
//   - Client: The registry adapter implementing the Client interface
//   - error: An error if the ecosystem is not supported or adapter creation fails
//
// Example:
//
//	client, err := packageregistry.NewRegistryAdapter(packagev1.Ecosystem_ECOSYSTEM_NPM)
//	if err != nil {
//		log.Fatalf("failed to create registry adapter: %v", err)
//	}
func NewRegistryAdapter(ecosystem packagev1.Ecosystem) (Client, error) {
	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_NPM:
		return NewNpmAdapter()
	case packagev1.Ecosystem_ECOSYSTEM_PYPI:
		return NewPypiAdapter()
	case packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:
		return NewRubyAdapter()
	default:
		return nil, fmt.Errorf("unsupported ecosystem: %s", ecosystem)
	}
}
