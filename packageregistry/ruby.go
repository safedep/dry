package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/http"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type rubyAdapter struct{}
type rubyPublisherDiscovery struct{}
type rubyPackageDiscovery struct{}

var _ Client = (*rubyAdapter)(nil)

func NewRubyAdapter() (Client, error) {
	return &rubyAdapter{}, nil
}

func (na *rubyAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &rubyPackageDiscovery{}, nil
}

func (na *rubyAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &rubyPublisherDiscovery{}, nil
}

func (np *rubyPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	packageName := packageVersion.GetPackage().GetName()

	packageURL := fmt.Sprintf("https://rubygems.org/api/v1/gems/%s/owners.json", packageName)
	res, err := http.Get(packageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ruby package metadata %w", err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unable to fetch ruby package metadata, statusCode: %d", res.StatusCode)
	}

	defer res.Body.Close()

	var owners []rubyPublisherData
	err = json.NewDecoder(res.Body).Decode(&owners)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON response in ruby registry adapter %w", err)
	}

	if len(owners) < 1 {
		return nil, fmt.Errorf("no authors found for package %s", packageName)
	}

	publishers := make([]*Publisher, 0, len(owners))
	for _, author := range owners {
		publishers = append(publishers, &Publisher{
			Name:  author.Username,
			Email: author.Email,
		})
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

func (np *rubyPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	publisherURL := fmt.Sprintf("https://rubygems.org/api/v1/owners/%s/gems.json", publisher.Name)

	res, err := http.Get(publisherURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ruby publisher metadata %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("packages not found for author %w", err)
	}

	var gemObjects []gemObject
	err = json.NewDecoder(res.Body).Decode(&gemObjects)
	if err != nil {
		return nil, fmt.Errorf("error decoding JSON in ruby registry adapter: %w", err)
	}

	return nil, nil
}

func (np *rubyPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	packageURL := rubyAPIEndpointPackageURL(packageName)

	res, err := http.Get(packageURL)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}

	if res.StatusCode == 404 {
		return nil, ErrPackageNotFound
	}

	if res.StatusCode != 200 {
		return nil, ErrFailedToFetchPackage
	}

	defer res.Body.Close()

	var gemObject gemObject
	err = json.NewDecoder(res.Body).Decode(&gemObject)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgVersions, err := np.GetPackageVersions(packageName)
	if err != nil {
		return nil, err
	}

	pkg := &Package{
		Name:                gemObject.Name,
		Description:         gemObject.Description,
		SourceRepositoryUrl: gemObject.SourceCodeURL,
		Versions:            pkgVersions,
		Downloads: OptionalInt{
			Value: gemObject.TotalDownloads,
			Valid: gemObject.TotalDownloads > 0,
		},
		CreatedAt: gemObject.CreatedAt,
		Author: Publisher{
			Name: gemObject.Authors,
		},
	}

	return pkg, nil
}

func (np *rubyPackageDiscovery) GetPackageVersions(packageName string) ([]PackageVersionInfo, error) {
	packageURL := rubyAPIEndpointAllVersionsURL(packageName)

	res, err := http.Get(packageURL)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}

	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrPackageNotFound
	}

	if res.StatusCode != 200 {
		return nil, ErrFailedToFetchPackage
	}

	var versions []rubyVersion
	err = json.NewDecoder(res.Body).Decode(&versions)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgVersions := make([]PackageVersionInfo, 0, len(versions))
	for _, version := range versions {
		pkgVersions = append(pkgVersions, PackageVersionInfo{
			Version: version.Number,
		})
	}

	return pkgVersions, nil
}
