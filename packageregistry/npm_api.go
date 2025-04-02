package packageregistry

import (
	"time"
)

// NpmPackage represents the structure of the package metadata
type npmPackage struct {
	Author struct {
		Name string `json:"name"`
	} `json:"author"`
	Maintainers []struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"maintainers"`
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
