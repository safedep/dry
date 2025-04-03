package packageregistry

import "fmt"

// RUBY GEM API ENDPOINTS
// DOCS: https://guides.rubygems.org/rubygems-org-api-v2/

// We use v1 endpoint for this, as v2 endpoint requires version
// We can find all the version of the package rubyAPIEndpointAllVersionURL, then we can use v2 endpoint to get the package metadata, but result is same
func rubyAPIEndpointPackageURL(packageName string) string {
	return fmt.Sprintf("https://rubygems.org/api/v1/gems/%s.json", packageName)
}

func rubyAPIEndpointPackageWithVersionURL(packageName, version string) string {
	return fmt.Sprintf("https://rubygems.org/api/v2/gems/%s/versions/%s.json", packageName, version)
}

// Get all versions of a package
// V1 API, v2 does not support this
func rubyAPIEndpointAllVersionsURL(packageName string) string {
	return fmt.Sprintf("https://rubygems.org/api/v1/versions/%s.json", packageName)
}
