package packageregistry

import "time"

type goProxyPackageVersion struct {
	version string               `json:"Version"`
	time    time.Time            `json:"Time"`
	origin  goProxyPackageOrigin `json:"Origin"`
}
type goProxyPackageOrigin struct {
	vcs  string `json:"VCS"`
	url  string `json:"URL"`
	hash string `json:"Hash"`
	ref  string `json:"Ref"`
}
