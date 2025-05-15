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

// Gets the download count for a package in the specified period
// periodPoint can be "last-day", "last-week", "last-month", "last-year"
func npmAPIEndpointPackageDownloadsURL(packageName string, periodPoint string) string {
	return fmt.Sprintf("https://api.npmjs.org/downloads/point/%s/%s", periodPoint, packageName)
}
