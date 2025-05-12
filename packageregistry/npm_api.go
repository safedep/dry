package packageregistry

import (
	"encoding/json"
	"time"
)

// npmPackage represents a package in the NPM registry
// Endpoint:
// - GET https://registry.npmjs.org/<packageName>
// Docs: https://github.com/npm/registry/blob/main/docs/REGISTRY-API.md#package
type npmPackage struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Versions    map[string]npmPackageVersion `json:"versions"`
	DistTags    npmPackageDistTags           `json:"dist-tags"`
	Author      npmPackageAuthor             `json:"author"`
	Repository  npmPackageRepository         `json:"repository"`
	Maintainers []npmPackageAuthor           `json:"maintainers"`
	Time        npmPackageTime               `json:"time"`
}

// Docs: https://github.com/npm/registry/blob/main/docs/REGISTRY-API.md#version
type npmPackageVersion struct {
	Version string `json:"version"`
}

type npmPackageDistTags struct {
	Latest string `json:"latest"`
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

	return ErrFailedToParsePackage
}

type npmPackageRepository struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}

type npmPackageTime struct {
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

type npmPackageVersionInfo struct {
	Maintainers     []npmPackageAuthor `json:"maintainers"`
	Dependencies    map[string]string  `json:"dependencies"`
	DevDependencies map[string]string  `json:"devDependencies"`
}

// npmPublisherRecord represents the response from the NPM API for packages with author
// Endpoint:
// - GET: https://registry.npmjs.org/-/v1/search?text=author:<publisherName>
type npmPublisherRecord struct {
	Objects []npmPublisherRecordPackage `json:"objects"`
	Total   uint32                      `json:"total"`
}

type npmPublisherRecordPackage struct {
	Package npmPublisherRecordPackageDetails `json:"package"`
}

// npmPublisherRecordPackageDetails represents the details of a package in the NPM publisher API
// But This only contains the name of the package, since we are going to fetch the details from the package API
// Beause current data only contains the latest version of the package, we want all version
type npmPublisherRecordPackageDetails struct {
	Name string `json:"name"`
}

// https://api.npmjs.org/downloads/point/last-year/react
type npmDownloadObject struct {
	Downloads uint64 `json:"downloads"`
}
