package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type pypiAdapter struct{}

// Verify that pypiAdapter implements the Client interface
var _ Client = (*pypiAdapter)(nil)

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

	packageURL := pypiAPIEndpointPackageWithVersionURL(packageName, version)
	res, err := httpClient().Get(packageURL)
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

	var pypipkg pypiPackage
	err = json.NewDecoder(res.Body).Decode(&pypipkg)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	author := parsePypiAuthor(
		pypipkg.Info.Author,
		pypipkg.Info.AuthorEmail,
		true,
	)
	if author.Name == "" && author.Email == "" {
		return nil, ErrAuthorNotFound
	}

	publishers := []Publisher{author}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

// Pypi does not support getting packages by publisher
func (np *pypiPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	return nil, ErrNoPackagesFound
}

func (np *pypiPackageDiscovery) GetPackageDependencies(packageName string,
	packageVersion string) (*PackageDependencyList, error) {
	return nil, fmt.Errorf("dependency resolution is not supported for PyPI adapter")
}

func (np *pypiPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	url := pypiAPIEndpointPackageURL(packageName)

	res, err := httpClient().Get(url)
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

	var pypipkg pypiPackage
	err = json.NewDecoder(res.Body).Decode(&pypipkg)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgVersions := make([]PackageVersionInfo, 0)
	for release := range pypipkg.Releases {
		pkgVersions = append(pkgVersions, PackageVersionInfo{
			Version: release,
		})
	}

	maintainers := make([]Publisher, 0)
	maintainer := parsePypiAuthor(
		pypipkg.Info.Maintainer,
		pypipkg.Info.MaintainerEmail,
		true,
	)
	if maintainer.Name != "" || maintainer.Email != "" {
		maintainers = append(maintainers, maintainer)
	}

	author := parsePypiAuthor(
		pypipkg.Info.Author,
		pypipkg.Info.AuthorEmail,
		true,
	)

	sourceGitURL, err := getNormalizedGitURL(pypipkg.Info.ProjectURLs.Source)
	if err != nil {
		return nil, err
	}

	pkg := Package{
		Name:                pypipkg.Info.Name,
		Description:         pypipkg.Info.Description,
		SourceRepositoryUrl: sourceGitURL,
		Author:              author,
		Maintainers:         maintainers,
		LatestVersion:       pypipkg.Info.LatestVersion,
		Versions:            pkgVersions,
		// No official way to get downloads
		// Though, we can use pypi.tech
		// https://api.pepy.tech/api/v2/projects/requests
		// But it require API key
		Downloads: OptionalInt{
			Valid: false,
			Value: 0,
		},
	}

	return &pkg, nil
}

func (np *pypiPackageDiscovery) GetPackageDownloadStats(packageName string) (DownloadStats, error) {
	return DownloadStats{}, fmt.Errorf("download stats is not supported for PyPI adapter")
}

func parsePypiAuthor(authorName, authorEmail string, splitEmailIntoName bool) Publisher {
	name := strings.TrimSpace(authorName)
	email := strings.TrimSpace(authorEmail)

	if name == "" && email == "" {
		return Publisher{}
	}

	if splitEmailIntoName {
		if parsed, err := mail.ParseAddress(email); err == nil {
			if name == "" {
				name = strings.TrimSpace(parsed.Name)
			}
			email = strings.TrimSpace(parsed.Address)
		}
	}

	return Publisher{
		Name:  name,
		Email: email,
	}
}
