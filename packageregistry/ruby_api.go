package packageregistry

import "time"

type rubyPublisherData struct {
	Username string `json:"handle"`
	Email    string `json:"email"`
}

type gemObject struct {
	Name             string    `json:"name"`
	Description      string    `json:"info"`
	CreatedAt        time.Time `json:"created_at"`
	Downloads        int       `json:"downloads"`
	Version          string    `json:"version"`
	VersionDownloads int       `json:"version_downloads"`
	VersionCreatedAt time.Time `json:"version_created_at"`
	SourceUrl        string    `json:"source_code_uri"`
	HomepageUrl      string    `json:"homepage_uri"`
	Sha              string    `json:"sha"`
}
