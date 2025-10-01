package packageregistry

import "time"

// cratesPackage represents the response from the Crates.io API for a package (crate).
// API endpoint: https://crates.io/api/v1/crates/{crate_name}
type cratesPackage struct {
	Package    cratesPackageInfo `json:"crate"`
	Versions   []cratesVersion   `json:"versions"`
	Keywords   []cratesKeyword   `json:"keywords"`
	Categories []cratesCategory  `json:"categories"`
}

// cratesPackageInfo contains basic information about a crate
type cratesPackageInfo struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Downloads        int       `json:"downloads"`
	Documentation    string    `json:"documentation"`
	Repository       string    `json:"repository"`
	Homepage         string    `json:"homepage"`
	MaxVersion       string    `json:"max_version"`
	MaxStableVersion string    `json:"max_stable_version"`
	NewestVersion    string    `json:"newest_version"`
}

// cratesVersion contains information about a specific version of a crate
type cratesVersion struct {
	ID        int       `json:"id"`
	Version   string    `json:"num"`
	Downloads int       `json:"downloads"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Yanked    bool      `json:"yanked"`
	License   string    `json:"license"`
	CrateSize int       `json:"crate_size"`
}

// cratesKeyword represents a keyword associated with a crate
type cratesKeyword struct {
	ID   string `json:"id"`
	Name string `json:"keyword"`
}

// cratesCategory represents a category associated with a crate
type cratesCategory struct {
	ID          string `json:"id"`
	Name        string `json:"category"`
	Description string `json:"description"`
}

// cratesOwners represents the response from the owners API endpoint
// API endpoint: https://crates.io/api/v1/crates/{crate_name}/owners
type cratesOwners struct {
	Users []cratesUser `json:"users"`
}

type cratesUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Url   string `json:"url"`
	Kind  string `json:"kind"`
}

// cratesSearchResults represents the response from the search API endpoint
// API endpoint: https://crates.io/api/v1/crates?q={query}
type cratesSearchResults struct {
	Crates []cratesPackageInfo `json:"crates"`
	Meta   cratesSearchMeta    `json:"meta"`
}

type cratesSearchMeta struct {
	Total    int    `json:"total"`
	NextPage string `json:"next_page"`
	PrevPage string `json:"prev_page"`
}

type crateDependency struct {
	Crate string `json:"crate_id"`
	Req   string `json:"req"`
	Kind  string `json:"kind"`
}

type crateDependenciesResponse struct {
	Dependencies []crateDependency `json:"dependencies"`
}
