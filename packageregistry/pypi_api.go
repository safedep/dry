package packageregistry

// PyPI API
// Docs: https://docs.pypi.org/api/json/

// pypiPackage represents the response from the PyPI API for a package.
type pypiPackage struct {
	Info     pypiPackageInfo `json:"info"`
	Releases map[string]any  `json:"releases"`
}

type pypiPackageInfo struct {
	Name            string `json:"name"`
	Description     string `json:"summary"`
	LatestVersion   string `json:"version"`
	PackageURL      string `json:"package_url"`
	Author          string `json:"author"`
	AuthorEmail     string `json:"author_email"`
	Maintainer      string `json:"maintainer"`
	MaintainerEmail string `json:"maintainer_email"`
}
