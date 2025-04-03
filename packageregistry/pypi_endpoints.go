package packageregistry

import "fmt"

func pypiPackageURL(packageName string) string {
	return fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
}

func pypiPackageWithVersionURL(packageName, version string) string {
	return fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", packageName, version)
}
