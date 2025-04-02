package packageregistry

import (
	"time"
)

// npmPackage represents a package in the NPM registry
// Endpoint:
// - GET https://registry.npmjs.org/<packageName>
type npmPackage struct {
	Name         string               `json:"name"`
	Versions     []npmPackageVersion  `json:"versions"`
	Time         npmPackageTime       `json:"time"`
	Bugs         npmPackageBugs       `json:"bugs"`
	Author       npmPackageAuthor     `json:"author"`
	License      string               `json:"license"`
	Homepage     string               `json:"homepage"`
	Keywords     []string             `json:"keywords"`
	Repository   npmPackageRepository `json:"repository"`
	Description  string               `json:"description"`
	Contributors []npmPackageAuthor   `json:"contributors"`
	Maintainers  []npmPackageAuthor   `json:"maintainers"`
	Users        []string             `json:"users"`
}

type npmPackageVersion struct {
	Name            string               `json:"name"`
	Version         string               `json:"version"`
	Description     string               `json:"description"`
	Deprecated      string               `json:"deprecated"`
	Keywords        []string             `json:"keywords"`
	Author          npmPackageAuthor     `json:"author"`
	Contributors    []npmPackageAuthor   `json:"contributors"`
	Dist            npmPackageDist       `json:"dist"`
	Dependencies    map[string]string    `json:"dependencies"`
	DevDependencies map[string]string    `json:"devDependencies"`
	Repository      npmPackageRepository `json:"repository"`
}

type npmPackageAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type npmPackageRepository struct {
	Url  string `json:"url"`
	Type string `json:"type"`
}

type npmPackageDist struct {
	Shasum     string                `json:"shasum"`
	Tarball    string                `json:"tarball"`
	Integrity  string                `json:"integrity"`
	Signatures []npmPackageSignature `json:"signatures"`
}

type npmPackageSignature struct {
	Sig   string `json:"sig"`
	Keyid string `json:"keyid"`
}

type npmPackageTime struct {
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

type npmPackageBugs struct {
	Url string `json:"url"`
}

// npmPublisherRecord represents the response from the NPM publisher API
// Endpoint:
// - GET: https://registry.npmjs.org/-/v1/search?text=author:<publisherName>
type npmPublisherRecord struct {
	Objects []npmPublisherRecordPackage `json:"objects"`
	Total   uint32                      `json:"total"`
}

type npmPublisherRecordPackage struct {
	Package     npmPackage          `json:"package"`
	Downloads   npmPackageDownloads `json:"downloads"`
	Dependents  uint32              `json:"dependents"`
	UpdatedAt   time.Time           `json:"updated"`
	SearchScore float64             `json:"searchScore"`
	Score       npmPackageScore     `json:"score"`
	Flags       npmPackageFlags     `json:"flags"`
}

type npmPublisherRecordPackagePackage struct {
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
