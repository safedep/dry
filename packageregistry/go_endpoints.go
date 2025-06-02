package packageregistry

import "fmt"

// Public Go Module Proxy: proxy.golang.org
// Protocol (Endpoint) Docs: https://go.dev/ref/mod#module-proxy

func goProxyAPIEndpointPackageLatestVersionURL(packageName string) string {
	return fmt.Sprintf("https://proxy.golang.org/%s/@latest", packageName)
}

func goProxyAPIEndpointPackageListAllVersions(packageName string) string {
	return fmt.Sprintf("https://proxy.golang.org/%s/@v/list", packageName)
}

func goProxyAPIEndpointGetPackageModFileFromVersion(packageName, packageVersion string) string {
	return fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.mod", packageName, packageVersion)
}
