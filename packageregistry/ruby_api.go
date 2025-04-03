package packageregistry

import "time"

type gemObject struct {
	Name             string    `json:"name"`
	TotalDownloads   uint64    `json:"downloads"`
	LatestVersion    string    `json:"version"`
	VersionCreatedAt time.Time `json:"version_created_at"`
	VersionDownloads uint64    `json:"version_downloads"`
	Authors          string    `json:"authors"`
	Description      string    `json:"info"`
	ProjectURI       string    `json:"project_uri"`
	SourceCodeURL    string    `json:"source_code_uri"`
	CreatedAt        time.Time `json:"created_at"`
}

type rubyPublisherData struct {
	Username string `json:"handle"`
	Email    string `json:"email"`
}

// ruby version data
// API (sample): https://rubygems.org/api/v1/versions/rails.json
// We only need version numbers, so we extract this only
type rubyVersion struct {
	Number string `json:"number"`
}
