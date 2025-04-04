package packageregistry

import "fmt"

// Npm API Endpoints
// Docs: https://github.com/npm/registry/blob/main/docs/REGISTRY-API.md

func npmAPIEndpointPackageURL(packageName string) string {
	return fmt.Sprintf("https://registry.npmjs.org/%s", packageName)
}

func npmAPIEndpointPackageWithVersionURL(packageName, version string) string {
	return fmt.Sprintf("https://registry.npmjs.org/%s/%s", packageName, version)
}

func npmAPIEndpointPackageSearchWithAuthorURL(author string) string {
	return fmt.Sprintf("https://registry.npmjs.org/-/v1/search?text=author:%s", author)
}

func npmAPIEndpointPackageDownloadsURL(packageName string) string {
	return fmt.Sprintf("https://api.npmjs.org/downloads/point/last-year/%s", packageName)
}
