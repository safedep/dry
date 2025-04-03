package packageregistry

import (
	"encoding/json"
	"time"
)

// Npm API
// Docs: https://github.com/npm/registry/blob/main/docs/REGISTRY-API.md

// npmPackage represents a package in the NPM registry
// Endpoint:
// - GET https://registry.npmjs.org/<packageName>
// Docs: https://github.com/npm/registry/blob/main/docs/REGISTRY-API.md#package
type npmPackage struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Versions    []npmPackageVersion  `json:"versions"`
	Author      npmPackageAuthor     `json:"author"`     // Can be string or object
	Repository  npmPackageRepository `json:"repository"` // Can be string or object
	Maintainers []npmPackageAuthor   `json:"maintainers"`
	Time        npmPackageTime       `json:"time"` // Time is always present
}

// Docs: https://github.com/npm/registry/blob/main/docs/REGISTRY-API.md#version
type npmPackageVersion struct {
	Version string `json:"version"`
}

// Throught registry docs....
// author can be object with name, email, and or url of author as listed in package.json
type npmPackageAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Url   string `json:"url"`
}

// Custom unmarshal for npmPackageAuthor, because the type can be string or object
func (a *npmPackageAuthor) UnmarshalJSON(data []byte) error {
	// try to string first
	var authorUrl string
	if err := json.Unmarshal(data, &authorUrl); err == nil {
		a.Url = authorUrl
		return nil
	}

	// try to object next
	var authorObject struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.Unmarshal(data, &authorObject); err == nil {
		a.Name = authorObject.Name
		a.Email = authorObject.Email
		return nil
	}

	return ErrFailedToParseNpmPackage
}

type npmPackageRepository struct {
	// Internally, we use url as a string
	url string
}

// Custom unmarshal for npmPackageRepository, because the type can be string or object
func (r *npmPackageRepository) UnmarshalJSON(data []byte) error {
	type npmPackageRepositoryType struct {
		Url  string `json:"url"`
		Type string `json:"type"`
	}

	// Try to string first
	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err == nil {
		r.url = stringValue
		return nil
	}

	// Try to object next
	var objectValue npmPackageRepositoryType
	if err := json.Unmarshal(data, &objectValue); err == nil {
		r.url = objectValue.Url
		return nil
	}

	return ErrFailedToParseNpmPackage
}

type npmPackageTime struct {
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

type npmPackageMaintainerInfo struct {
	Maintainers []npmPackageAuthor `json:"maintainers"`
}

// npmPublisherRecord represents the response from the NPM API for packages with author
// Endpoint:
// - GET: https://registry.npmjs.org/-/v1/search?text=author:<publisherName>
type npmPublisherRecord struct {
	Objects []npmPublisherRecordPackage `json:"objects"`
	Total   uint32                      `json:"total"`
}

type npmPublisherRecordPackage struct {
	Package     npmPublisherRecordPackageDetails `json:"package"`
	Downloads   npmPackageDownloads              `json:"downloads"`
	Dependents  uint32                           `json:"dependents"`
	UpdatedAt   time.Time                        `json:"updated"`
	SearchScore float64                          `json:"searchScore"`
	Score       npmPackageScore                  `json:"score"`
	Flags       npmPackageFlags                  `json:"flags"`
}

// npmPublisherRecordPackageDetails represents the details of a package in the NPM publisher API
// But This only contains the name of the package, since we are going to fetch the details from the package API
// Beause current data only contains the latest version of the package, we want all version
type npmPublisherRecordPackageDetails struct {
	Name string `json:"name"`
}

type npmPackageDownloads struct {
	Monthly uint32 `json:"monthly"`
	Weekly  uint32 `json:"weekly"`
}

type npmPackageScore struct {
	Final  float64        `json:"final"`
	Detail npmScoreDetail `json:"detail"`
}

type npmScoreDetail struct {
	Quality     float64 `json:"quality"`
	Popularity  float64 `json:"popularity"`
	Maintenance float64 `json:"maintenance"`
}

type npmPackageFlags struct {
	Insecure bool `json:"insecure"`
}
