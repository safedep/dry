package packageregistry

import "time"

type goProxyPackageVersion struct {
	Version string               `json:"Version"`
	Time    time.Time            `json:"Time"`
	Origin  goProxyPackageOrigin `json:"Origin"`
}
type goProxyPackageOrigin struct {
	VCS  string `json:"VCS"`
	URL  string `json:"URL"`
	Hash string `json:"Hash"`
	Ref  string `json:"Ref"`
}
