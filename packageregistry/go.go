package packageregistry

import (
	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"encoding/json"
	"golang.org/x/mod/modfile"
	"io"
	"net/http"
	"strings"
)

type goAdapter struct{}
type goPublisherDiscovery struct{}
type goPackageDiscovery struct{}

// Verify that goAdapter implements the Client interface
var _ Client = (*goAdapter)(nil)

// NewGoAdapter creates a new Go registry adapter
func NewGoAdapter() (Client, error) {
	return &goAdapter{}, nil
}

func (na *goAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &goPublisherDiscovery{}, nil
}

func (na *goAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &goPackageDiscovery{}, nil
}

func (g goPublisherDiscovery) GetPackagePublisher(_ *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	return nil, nil
}

func (g goPublisherDiscovery) GetPublisherPackages(_ Publisher) ([]*Package, error) {
	return nil, nil
}

func (g goPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	url := goProxyAPIEndpointPackageLatestVersionURL(packageName)

	res, err := http.Get(url)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, ErrFailedToFetchPackage
	}

	var goPkgVersion goProxyPackageVersion
	if err := json.NewDecoder(res.Body).Decode(&goPkgVersion); err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgAllVersions, err := g.getPackageAllVersion(packageName)
	if err != nil {
		return nil, err
	}

	return &Package{
		Name:                packageName,
		Versions:            pkgAllVersions,
		LatestVersion:       goPkgVersion.Version,
		SourceRepositoryUrl: goPkgVersion.Origin.URL,
		CreatedAt:           goPkgVersion.Time,
	}, nil
}

func (g goPackageDiscovery) GetPackageDependencies(packageName string, packageVersion string) (*PackageDependencyList, error) {
	url := goProxyAPIEndpointGetPackageModFileFromVersion(packageName, packageVersion)
	res, err := http.Get(url)

	if err != nil {
		return nil, ErrFailedToFetchPackage
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, ErrFailedToFetchPackage
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	// The result from this API is a TEXT file (literally go.mod file) - we have to parse it
	// ParseLax is a safer version of Parse, which will ignore unknown statements
	file, err := modfile.ParseLax("go.mod", data, nil)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	deps := make([]PackageDependencyInfo, 0)

	for _, req := range file.Require {
		deps = append(deps, PackageDependencyInfo{
			Name:        req.Mod.Path,
			VersionSpec: req.Mod.Version,
		})
	}

	return &PackageDependencyList{
		Dependencies: deps,
	}, nil
}

func (g goPackageDiscovery) GetPackageDownloadStats(packageName string) (DownloadStats, error) {
	return DownloadStats{}, nil
}

func (g goPackageDiscovery) getPackageAllVersion(packageName string) ([]PackageVersionInfo, error) {
	url := goProxyAPIEndpointPackageListAllVersions(packageName)

	res, err := http.Get(url)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrPackageNotFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, ErrFailedToFetchPackage
	}

	// The Response of this API is a TEXT with one version per line - We need to parse it
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgVersions := make([]PackageVersionInfo, 0)

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line := strings.TrimSpace(line)
		if line != "" {
			pkgVersions = append(pkgVersions, PackageVersionInfo{
				Version: line,
			})
		}
	}

	return pkgVersions, nil
}
