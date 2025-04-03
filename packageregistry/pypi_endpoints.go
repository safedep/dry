package packageregistry

import "fmt"

func pypiPackageURL(packageName string) string {
	return fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
}
