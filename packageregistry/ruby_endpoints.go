package packageregistry

import "fmt"

// RUBY GEM API ENDPOINTS
// DOCS: https://guides.rubygems.org/rubygems-org-api-v2/

// We use v1 endpoint for this, as v2 endpoint requires version
// We can find all the version of the package rubyAPIEndpointAllVersionURL, then we can use v2 endpoint to get the package metadata, but result is same
func rubyAPIEndpointPackageURL(packageName string) string {
	return fmt.Sprintf("https://rubygems.org/api/v1/gems/%s.json", packageName)
}

func rubyAPIEndpointGetPublishersForPackageURL(packageName string) string {
	return fmt.Sprintf("https://rubygems.org/api/v1/gems/%s/owners.json", packageName)
}

// Get all versions of a package
// V1 API, v2 does not support this
func rubyAPIEndpointAllVersionsURL(packageName string) string {
	return fmt.Sprintf("https://rubygems.org/api/v1/versions/%s.json", packageName)
}

func rubyAPIEndpointPackageByAuthorURL(author string) string {
	return fmt.Sprintf("https://rubygems.org/api/v1/owners/%s/gems.json", author)
}
