package packageregistry

import (
	"net/http"
	"time"
)

// userAgent identifies our client to package registries. Some registries
// (e.g. crates.io) reject requests using the default Go user-agent with a
// 403, so we set a descriptive one that links back to the project.
const userAgent = "safedep-dry (https://github.com/safedep/dry)"

// userAgentTransport sets a User-Agent header on every request that does not
// already carry one.
type userAgentTransport struct {
	base http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		// Clone to avoid mutating a request the caller may reuse.
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", userAgent)
	}
	return t.base.RoundTrip(req)
}

// httpClient returns a new http.Client with our opinionated
// defaults. If required, we need to implement a package specific
// configuration / fine tuning.
func httpClient() *http.Client {
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &userAgentTransport{base: http.DefaultTransport},
	}
}
