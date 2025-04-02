package packageregistry

import (
	"fmt"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// Holds package registry specific information about verification
// status of a publisher in the registry.
type PublisherVerificationStatus struct {
	IsVerified bool `json:"is_verified"`
}

// Represents an individual or an organization having an account with
// a package registry for publishing package artifacts.
type Publisher struct {
	Name  string `json:"name"`
	Email string `json:"email"`

	// Verification may or may not be supported by all
	// package registries. Hence its optional
	VerificationStatus *PublisherVerificationStatus `json:"verification_status"`

	// Any other relevant information for a publisher
}

// Publisher information for a given package in a registry
type PackagePublisherInfo struct {
	Publishers []*Publisher `json:"publishers"`
}

// Represents a version of a package in a package registry.
type PackageVersionInfo struct {
	// The version of the project.
	Version string `json:"version"`

	// The release & update date of the version
	// Both can be same if the version is immutable.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Other version related metrics. May not be available
	// for all package registries.
	Downloads int `json:"downloads"`
}

// PackageInfo contains stats and metadata of the package
type PackageInfo struct {
	Stars     int `json:"stars"`
	Forks     int `json:"forks"`
	Downloads int `json:"downloads"`
}

// Example: `requests` in PyPI, `rails` in RubyGems.
type Package struct {
	// The name of the project.
	Name string `json:"name"`

	// The source repository URL for the project.
	SourceRepositoryUrl string `json:"source_repository_url"`

	// The registry url for the Package
	PackageUrl string `json:"package_url"`

	// Homepage Url for the package
	HomepageUrl string `json:"homepage_url"`

	// The project description.
	Description string `json:"description"`

	// Published versions of the project.
	Versions []PackageVersionInfo `json:"versions"`

	// Package metadata
	PackageInfo PackageInfo `json:"package_info"`

	// Important timestamps
	CreatedAt time.Time `json:"created_at"`
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
