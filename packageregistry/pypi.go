package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/http"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type pypiAdapter struct{}
type pypiPublisherDiscovery struct{}
type pypiPackageDiscovery struct{}

// Verify that pypiAdapter implements the Client interface
var _ Client = (*pypiAdapter)(nil)

func NewPypiAdapter() (Client, error) {
	return &pypiAdapter{}, nil
}

func (na *pypiAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &pypiPublisherDiscovery{}, nil
}

func (na *pypiAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &pypiPackageDiscovery{}, nil
}

func (np *pypiPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	packageName := packageVersion.GetPackage().GetName()
	version := packageVersion.GetVersion()

	packageURL := fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", packageName, version)
	res, err := http.Get(packageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pypi package metadata %w", err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("unable to fetch pypi package metadata, statusCode: %d", res.StatusCode)
	}

	defer res.Body.Close()

	var pypipkg pypiPackage
	err = json.NewDecoder(res.Body).Decode(&pypipkg)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON response in pypi registry adapter %w", err)
	}

	if pypipkg.Info.Author == "" && pypipkg.Info.AuthorEmail == "" {
		return nil, fmt.Errorf("no maintainers found for package %s", packageName)
	}
	// TODO: Sometimes AuthorEmail does not have proper email in that case email needs to parsed and retrieved
	publishers := []*Publisher{
		{
			Name:  pypipkg.Info.Author,
			Email: pypipkg.Info.AuthorEmail,
		},
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

func (np *pypiPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	publisherPackages := []*Package{}

	return publisherPackages, nil
}

func (np *pypiPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	url := pypiPackageURL(packageName)

	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pypi package metadata %w", err)
	}

	if res.StatusCode == 404 {
		return nil, ErrPackageNotFound
	}

	defer res.Body.Close()

	return nil, nil
}
