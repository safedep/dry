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

	packageURL := fmt.Sprintf(string(npmEndpointPackageWithVersion), packageName, version)
	res, err := http.Get(packageURL)
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

	// Publishers in case of NPM are all the Maintainers of the package
	// Hense we only need to extract the Maintainers from the response
	var npmpkg npmPackageMaintainerInfo
	err = json.NewDecoder(res.Body).Decode(&npmpkg)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	publishers := make([]*Publisher, len(npmpkg.Maintainers))

	for i, maintainer := range npmpkg.Maintainers {
		publishers[i] = &Publisher{
			Name:  maintainer.Name,
			Email: maintainer.Email,
			Url:   maintainer.Url,
		}
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

// GetPublisherPackages returns all the packages published by a given publisher
func (np *npmPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	publisherURL := fmt.Sprintf(string(npmEndpointSearchWithAuthor), publisher.Name)

	res, err := http.Get(publisherURL)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}
	defer res.Body.Close()

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
		pkg, err := getPackageDetails(obj.Package.Name)
		if err != nil {
			return nil, err
		}
		packages[i] = pkg
	}

	return packages, nil
}

func (np *npmPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	return getPackageDetails(packageName)
}

func getPackageDetails(packageName string) (*Package, error) {
	packageURL := fmt.Sprintf(string(npmEndpointPackage), packageName)

	res, err := http.Get(packageURL)
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

	pkg := Package{
		Name:                npmpkg.Name,
		Description:         npmpkg.Description,
		SourceRepositoryUrl: npmpkg.Repository.Url,
		Versions:            pkgVerions,
		CreatedAt:           npmpkg.Time.Created,
		UpdatedAt:           npmpkg.Time.Modified,
		Publisher: Publisher{
			Name:  npmpkg.Author.Name,
			Email: npmpkg.Author.Email,
			Url:   npmpkg.Author.Url,
		},
		Maintainers: pkgMaintainers,
	}

	return &pkg, nil
}
