package packageregistry

import "fmt"

func pypiAPIEndpointPackageURL(packageName string) string {
	return fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
}

func pypiAPIEndpointPackageWithVersionURL(packageName, version string) string {
	return fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", packageName, version)
}
