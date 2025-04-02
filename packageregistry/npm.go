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

	packageURL := fmt.Sprintf("https://registry.npmjs.org/%s/%s", packageName, version)
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
		}
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

func (np *npmPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	publisherURL := fmt.Sprintf("https://registry.npmjs.org/-/v1/search?text=author:%s", publisher.Name)

	res, err := http.Get(publisherURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch npm publisher metadata %w", err)
	}

	defer res.Body.Close()

	var pubRecord npmPublisherRecord
	err = json.NewDecoder(res.Body).Decode(&pubRecord)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON in npm registry adapter: %w", err)
	}

	// for _, obj := range pubRecord.Objects {
	// }
	return nil, nil
}

func (np *npmPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	packageURL := fmt.Sprintf("https://registry.npmjs.org/%s", packageName)

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
			Version:    version.Version,
			Depricated: version.Deprecated != "",
			Author: Publisher{
				Name:  version.Author.Name,
				Email: version.Author.Email,
			},
		})
	}

	pkgMaintainers := make([]Publisher, 0)
	for _, maintainer := range npmpkg.Maintainers {
		pkgMaintainers = append(pkgMaintainers, Publisher{
			Name:  maintainer.Name,
			Email: maintainer.Email,
		})
	}

	pkg := Package{
		Name:                npmpkg.Name,
		Description:         npmpkg.Description,
		SourceRepositoryUrl: npmpkg.Repository.Url,
		PackageUrl:          fmt.Sprintf("https://www.npmjs.com/package/%s", npmpkg.Name),
		HomepageUrl:         npmpkg.Homepage,
		Versions:            pkgVerions,
		CreatedAt:           npmpkg.Time.Created,
		UpdatedAt:           npmpkg.Time.Modified,
		Publisher: Publisher{
			Name:  npmpkg.Author.Name,
			Email: npmpkg.Author.Email,
		},
		Maintainers: pkgMaintainers,
		PackageInfo: PackageInfo{
			License: npmpkg.License,
		},
	}

	return &pkg, nil
}
