package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/http"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type npmAdapter struct{}
type npmPublisherDiscovery struct{}
type npmPackageDiscovery struct{}

// Verify that npmAdapter implements the Client interface
var _ Client = (*npmAdapter)(nil)

// NewNpmAdapter creates a new NPM registry adapter
func NewNpmAdapter() (Client, error) {
	return &npmAdapter{}, nil
}

func (na *npmAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &npmPublisherDiscovery{}, nil
}

func (na *npmAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &npmPackageDiscovery{}, nil
}

// GetPackagePublisher returns the publisher of a package
func (np *npmPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	packageName := packageVersion.GetPackage().GetName()
	version := packageVersion.GetVersion()

	npmpkg, err := npmGetPackageVersionDetails(packageName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get package version details: %w", err)
	}

	publishers := make([]Publisher, len(npmpkg.Maintainers))
	for i, maintainer := range npmpkg.Maintainers {
		publishers[i] = Publisher{
			Name:  maintainer.Name,
			Email: maintainer.Email,
			Url:   maintainer.Url,
		}
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

// GetPublisherPackages returns all the packages published by a given publisher
func (np *npmPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	url := npmAPIEndpointPackageSearchWithAuthorURL(publisher.Name)

	res, err := http.Get(url)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNoPackagesFound
	}

	if res.StatusCode != http.StatusOK {
		return nil, ErrFailedToFetchPackage
	}

	var pubRecord npmPublisherRecord
	err = json.NewDecoder(res.Body).Decode(&pubRecord)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	if len(pubRecord.Objects) == 0 {
		return nil, ErrNoPackagesFound
	}

	packages := make([]*Package, len(pubRecord.Objects))
	for i, obj := range pubRecord.Objects {
		pkg, err := npmGetPackageDetails(obj.Package.Name)
		if err != nil {
			return nil, err
		}
		packages[i] = pkg
	}

	return packages, nil
}

func (np *npmPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	return npmGetPackageDetails(packageName)
}

func (np *npmPackageDiscovery) GetPackageDependencies(packageName string, packageVersion string) (*PackageDependencyList, error) {
	npmpkg, err := npmGetPackageVersionDetails(packageName, packageVersion)
	if err != nil {
		return nil, err
	}

	dependencies := make([]PackageDependencyInfo, 0)
	for name, version := range npmpkg.Dependencies {
		dependencies = append(dependencies, PackageDependencyInfo{
			Name:        name,
			VersionSpec: version,
		})
	}

	devDependencies := make([]PackageDependencyInfo, 0)
	for name, version := range npmpkg.DevDependencies {
		devDependencies = append(devDependencies, PackageDependencyInfo{
			Name:        name,
			VersionSpec: version,
		})
	}

	return &PackageDependencyList{
		Dependencies:    dependencies,
		DevDependencies: devDependencies,
	}, nil
}

func npmGetPackageVersionDetails(packageName string, packageVersion string) (*npmPackageVersionInfo, error) {
	url := npmAPIEndpointPackageWithVersionURL(packageName, packageVersion)

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

	var npmpkg npmPackageVersionInfo
	err = json.NewDecoder(res.Body).Decode(&npmpkg)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	return &npmpkg, nil
}

func npmGetPackageDetails(packageName string) (*Package, error) {
	url := npmAPIEndpointPackageURL(packageName)

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

	var npmpkg npmPackage
	err = json.NewDecoder(res.Body).Decode(&npmpkg)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgVerions := make([]PackageVersionInfo, 0)
	for _, version := range npmpkg.Versions {
		pkgVerions = append(pkgVerions, PackageVersionInfo{
			Version: version.Version,
		})
	}

	pkgMaintainers := make([]Publisher, 0)
	for _, maintainer := range npmpkg.Maintainers {
		pkgMaintainers = append(pkgMaintainers, Publisher{
			Name:  maintainer.Name,
			Email: maintainer.Email,
			Url:   maintainer.Url,
		})
	}

	downloads, err := npmGetPackageDownloads(packageName)
	if err != nil {
		return nil, err
	}

	sourceGitURL, err := getNormalizedGitURL(npmpkg.Repository.Url)
	if err != nil {
		return nil, err
	}

	pkg := Package{
		Name:                npmpkg.Name,
		Description:         npmpkg.Description,
		SourceRepositoryUrl: sourceGitURL,
		Versions:            pkgVerions,
		LatestVersion:       npmpkg.DistTags.Latest,
		CreatedAt:           npmpkg.Time.Created,
		UpdatedAt:           npmpkg.Time.Modified,
		Downloads:           OptionalInt{Value: downloads, Valid: downloads > 0},
		Author: Publisher{
			Name:  npmpkg.Author.Name,
			Email: npmpkg.Author.Email,
			Url:   npmpkg.Author.Url,
		},
		Maintainers: pkgMaintainers,
	}

	return &pkg, nil
}

func npmGetPackageDownloads(packageName string) (uint64, error) {
	url := npmAPIEndpointPackageDownloadsURL(packageName)

	res, err := http.Get(url)
	if err != nil {
		return 0, ErrFailedToFetchPackage
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return 0, ErrFailedToFetchPackage
	}

	if res.StatusCode == http.StatusNotFound {
		return 0, ErrPackageNotFound
	}

	var downloadObject npmDownloadObject
	err = json.NewDecoder(res.Body).Decode(&downloadObject)
	if err != nil {
		return 0, ErrFailedToParsePackage
	}

	return downloadObject.Downloads, nil
}
