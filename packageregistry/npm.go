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

func (np *npmPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	packageName := packageVersion.GetPackage().GetName()
	version := packageVersion.GetVersion()

	packageURL := fmt.Sprintf("https://registry.npmjs.org/%s/%s", packageName, version)
	res, err := http.Get(packageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch npm package metadata %w", err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unable to fetch npm package metadata, statusCode: %d", res.StatusCode)
	}

	defer res.Body.Close()

	var npmpkg npmPackage
	err = json.NewDecoder(res.Body).Decode(&npmpkg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON response in npm registry adapter %w", err)
	}

	if npmpkg.Maintainers == nil {
		return nil, fmt.Errorf("no maintainers found for package %s", packageName)
	}

	publishers := make([]*Publisher, 0, len(npmpkg.Maintainers))
	for _, maintainer := range npmpkg.Maintainers {
		publishers = append(publishers, &Publisher{
			Name:  maintainer.Name,
			Email: maintainer.Email,
		})
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

	var pubObject npmPublisherObject
	err = json.NewDecoder(res.Body).Decode(&pubObject)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON in npm registry adapter: %w", err)
	}

	publisherPackages := []*Package{}
	for _, obj := range pubObject.Objects {
		pkg := Package{
			Name:                obj.Package.Name,
			SourceRepositoryUrl: obj.Package.Links.SourceRepository,
			PackageUrl:          obj.Package.Links.Npm,
			HomepageUrl:         obj.Package.Links.Homepage,
			Description:         obj.Package.Description,
			Versions: []PackageVersionInfo{
				{
					Version: obj.Package.Version,
				},
			},
			CreatedAt: obj.Package.Date,
			UpdatedAt: obj.Updated,
			PackageInfo: PackageInfo{
				Downloads: obj.Downloads.Monthly,
			},
		}

		publisherPackages = append(publisherPackages, &pkg)
	}

	if len(publisherPackages) < 1 {
		return nil, fmt.Errorf("Packages not found for author :%s", publisher.Name)
	}

	return publisherPackages, nil
}

func (np *npmPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	packageURL := fmt.Sprintf("https://registry.npmjs.org/%s", packageName)

	res, err := http.Get(packageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch npm package metadata %w", err)
	}

	defer res.Body.Close()

	var npmpkg npmPackage
	err = json.NewDecoder(res.Body).Decode(&npmpkg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON response in npm registry adapter %w", err)
	}

	pkg := Package{
		Name: npmpkg.Name,
	}

	return &pkg, nil
}
