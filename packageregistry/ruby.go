package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/http"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type rubyAdapter struct{}
type rubyPublisherDiscovery struct{}

func NewRubyAdapter() (Client, error) {
	return &rubyAdapter{}, nil
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

	publisherPackages := []*Package{}
	for _, obj := range gemObjects {
		pkg := Package{
			Name:                obj.Name,
			SourceRepositoryUrl: obj.SourceUrl,
			HomepageUrl:         obj.HomepageUrl,
			Description:         obj.Description,
			Versions: []PackageVersionInfo{
				{
					Version:   obj.Version,
					Downloads: obj.VersionDownloads,
					CreatedAt: obj.VersionCreatedAt,
				},
			},
			CreatedAt: obj.CreatedAt,
			PackageInfo: PackageInfo{
				Downloads: obj.Downloads,
			},
		}

		publisherPackages = append(publisherPackages, &pkg)
	}

	if len(publisherPackages) < 1 {
		return nil, fmt.Errorf("packages not found for author :%s", publisher.Name)
	}

	return publisherPackages, nil
}
