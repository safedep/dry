package packageregistry

import (
	"time"
)

// npmPackage represents a package in the NPM registry
// We convert the npm package to our own Package struct
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

type npmPublisherObject struct {
	Objects []struct {
		Downloads struct {
			Monthly int `json:"monthly"`
			Weekly  int `json:"weekly"`
		} `json:"downloads"`
		Updated time.Time             `json:"updated"`
		Package npmPackageVersionInfo `json:"package"`
	} `json:"objects"`
}

type npmPackageVersionInfo struct {
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Description string           `json:"description"`
	Publisher   maintainerInfo   `json:"publisher"`
	Maintainers []maintainerInfo `json:"maintainers"`
	License     string           `json:"license"`
	Date        time.Time        `json:"date"`
	Links       struct {
		Bugs             string `json:"bugs"`
		Npm              string `json:"npm"`
		SourceRepository string `json:"repository"`
		Homepage         string `json:"homepage"`
	} `json:"links"`
}

type maintainerInfo struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}
