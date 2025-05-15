package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/http"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type rubyAdapter struct{}

// Verify that rubyAdapter implements the Client interface
var _ Client = (*rubyAdapter)(nil)

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

	packageURL := rubyAPIEndpointGetPublishersForPackageURL(packageName)
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

	var owners []rubyPublisherData
	err = json.NewDecoder(res.Body).Decode(&owners)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	if len(owners) < 1 {
		return nil, ErrAuthorNotFound
	}

	publishers := make([]Publisher, 0, len(owners))
	for _, author := range owners {
		publishers = append(publishers, Publisher{
			Name:  author.Handle,
			Email: author.Email,
		})
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

func (np *rubyPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	publisherURL := rubyAPIEndpointPackageByAuthorURL(publisher.Name)

	res, err := http.Get(publisherURL)
	if err != nil {
		return nil, ErrFailedToFetchPackage
	}

	if res.StatusCode == 404 {
		return nil, ErrAuthorNotFound
	}

	if res.StatusCode != 200 {
		return nil, ErrFailedToFetchPackage
	}

	defer res.Body.Close()

	var gemObjects []gemObject
	err = json.NewDecoder(res.Body).Decode(&gemObjects)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	packages := make([]*Package, 0, len(gemObjects))

	for _, gemObject := range gemObjects {
		pkg, err := convertGemObjectToPackage(gemObject)
		if err != nil {
			return nil, err
		}

		packages = append(packages, pkg)
	}

	return packages, nil
}

func (np *rubyPackageDiscovery) GetPackageDependencies(packageName string,
	packageVersion string) (*PackageDependencyList, error) {
	return nil, fmt.Errorf("dependency resolution is not supported for Ruby adapter")
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

	return convertGemObjectToPackage(gemObject)
}

func convertGemObjectToPackage(gemObject gemObject) (*Package, error) {
	pkgVersions, err := getPackageVersions(gemObject.Name)
	if err != nil {
		return nil, err
	}

	sourceGitURL, err := getNormalizedGitURL(gemObject.SourceCodeURL)
	if err != nil {
		return nil, err
	}

	pkg := &Package{
		Name:                gemObject.Name,
		Description:         gemObject.Description,
		SourceRepositoryUrl: sourceGitURL,
		LatestVersion:       gemObject.LatestVersion,
		Versions:            pkgVersions,
		Downloads: DownloadStats{
			Daily:   0,
			Weekly:  0,
			Monthly: 0,
			Total:   gemObject.TotalDownloads,
		},
		CreatedAt: gemObject.CreatedAt,
		Author: Publisher{
			Name: gemObject.Authors,
		},
	}

	return pkg, nil
}

func getPackageVersions(packageName string) ([]PackageVersionInfo, error) {
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
