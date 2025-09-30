package packageregistry

import "fmt"

// Crates API Endpoints
// https://doc.rust-lang.org/cargo/reference/registry-index.html#index-format

var cratesBaseURL = "https://crates.io/api/v1"

func cratesAPIEndpointPackageURL(packageName string) string {
	return fmt.Sprintf("https://crates.io/api/v1/crates/%s", packageName)
}

func cratesAPIEndpointPackageWithVersionURL(packageName, version string) string {
	return fmt.Sprintf("https://crates.io/api/v1/crates/%s/%s", packageName, version)
}

func cratesAPIEndpointPackageDependencies(packageName, version string) string {
	return fmt.Sprintf("%s/dependencies", cratesAPIEndpointPackageWithVersionURL(packageName, version))
}

func cratesAPIEndpointPackageSearchWithOwners(packageName string) string {
	return fmt.Sprintf("%s/owners", cratesAPIEndpointPackageURL(packageName))
}
