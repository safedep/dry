package packageregistry

import (
	"net/http"
	"time"
)

// httpClient returns a new http.Client with our opinionated
// defaults. If required, we need to implement a package specific
// configuration / fine tuning.
func httpClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}
