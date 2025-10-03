package packageregistry

import (
	"encoding/json"
	"fmt"
	"net/http"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type cratesAdapter struct{}
type cratesPublisherDiscovery struct{}
type cratesPackageDiscovery struct{}

// Verify that cratesAdapter implements the Client interface
var _ Client = (*cratesAdapter)(nil)

// NewCratesAdapter creates a new Crates.io registry adapter
func NewCratesAdapter() (Client, error) {
	return &cratesAdapter{}, nil
}

func (ca *cratesAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &cratesPublisherDiscovery{}, nil
}

func (ca *cratesAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &cratesPackageDiscovery{}, nil
}

// GetPackagePublisher returns the publishers of a package
func (cp *cratesPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	packageName := packageVersion.GetPackage().GetName()

	url := cratesAPIEndpointPackageSearchWithOwners(packageName)

	res, err := httpClient().Get(url)
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

	var owners cratesOwners
	err = json.NewDecoder(res.Body).Decode(&owners)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	publishers := make([]Publisher, 0)

	// Add individual users
	for _, user := range owners.Users {
		publishers = append(publishers, Publisher{
			ID:   user.ID,
			Name: user.Name,
			Url:  user.Url,
			// Mail is not provided by Crates.io API
			// Email: "",
		})
	}

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

// GetPublisherPackages returns all packages published by a given publisher
func (cp *cratesPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	if publisher.ID == 0 {
		return nil, ErrAuthorNotFound
	}

	// Will traverse maximum of 5 pages from the publisher packages
	const MAX_PAGES = 5

	// Max limit by crates.io API is 100 for a single page
	const MAX_PACKAGES_PER_PAGE = 100

	query := fmt.Sprintf("?user_id=%d&page=1&per_page=%d", publisher.ID, MAX_PACKAGES_PER_PAGE)

	page := 0
	var allSearchResults []cratesPackageInfo

	for query != "" && page < MAX_PAGES {
		// The crates API provides the query separator "?" in the `next_page` field
		url := cratesAPIEndpointPackageWithQuery(query)
		res, err := httpClient().Get(url)
		if err != nil {
			return nil, ErrFailedToFetchPackage
		}

		if res.StatusCode != http.StatusOK {
			return nil, ErrFailedToFetchPackage
		}

		var searchResults cratesSearchResults
		err = json.NewDecoder(res.Body).Decode(&searchResults)

		// Close response body explicitly after reading
		res.Body.Close()

		if err != nil {
			return nil, ErrFailedToParsePackage
		}

		allSearchResults = append(allSearchResults, searchResults.Crates...)
		if searchResults.Meta.NextPage == "" {
			break
		}
		page++
		query = searchResults.Meta.NextPage
	}

	if len(allSearchResults) == 0 {
		return nil, ErrPackageNotFound
	}

	packages := make([]*Package, len(allSearchResults))
	for i, crate := range allSearchResults {
		pkg, err := cratesGetPackageDetails(crate.Name)
		if err != nil {
			return nil, err
		}
		packages[i] = pkg
	}

	return packages, nil
}

func (cp *cratesPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	return cratesGetPackageDetails(packageName)
}

func (cp *cratesPackageDiscovery) GetPackageDependencies(packageName, packageVersion string) (*PackageDependencyList, error) {
	url := cratesAPIEndpointPackageDependencies(packageName, packageVersion)

	res, err := httpClient().Get(url)
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

	var dependenciesResp crateDependenciesResponse
	err = json.NewDecoder(res.Body).Decode(&dependenciesResp)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	dependencies := make([]PackageDependencyInfo, 0)
	devDependencies := make([]PackageDependencyInfo, 0)

	for _, dep := range dependenciesResp.Dependencies {
		depInfo := PackageDependencyInfo{
			Name:        dep.Crate,
			VersionSpec: dep.Req,
		}

		// In Rust, dependencies can be normal, dev, or build
		if dep.Kind == "dev" {
			devDependencies = append(devDependencies, depInfo)
		} else {
			dependencies = append(dependencies, depInfo)
		}
	}

	return &PackageDependencyList{
		Dependencies:    dependencies,
		DevDependencies: devDependencies,
	}, nil
}

func (cp *cratesPackageDiscovery) GetPackageDownloadStats(packageName string) (DownloadStats, error) {
	// Get the package to extract download counts
	pkg, err := cratesGetPackageDetails(packageName)
	if err != nil {
		return DownloadStats{}, err
	}

	// Crates.io doesn't provide daily/weekly/monthly download stats via API
	// So we'll only return the total
	downloads := uint64(0)
	if pkg.Downloads.Valid {
		downloads = pkg.Downloads.Value
	}

	return DownloadStats{
		Total: downloads,
		// These fields don't have direct equivalents in the Crates.io API
		Daily:   0,
		Weekly:  0,
		Monthly: 0,
	}, nil
}

func cratesGetPackageDetails(packageName string) (*Package, error) {
	pkgUrl := cratesAPIEndpointPackageURL(packageName)

	res, err := httpClient().Get(pkgUrl)
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

	var crateResp cratesPackage
	err = json.NewDecoder(res.Body).Decode(&crateResp)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	pkgVersions := make([]PackageVersionInfo, 0)
	for _, version := range crateResp.Versions {
		if !version.Yanked {
			pkgVersions = append(pkgVersions, PackageVersionInfo{
				Version:     version.Version,
				PublishedAt: &version.CreatedAt,
			})
		}
	}

	ownersUrl := cratesAPIEndpointPackageSearchWithOwners(packageName)
	ownersRes, err := httpClient().Get(ownersUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch owners for %s: %w", packageName, err)
	}

	var owners cratesOwners
	if ownersRes.StatusCode == http.StatusOK {
		defer ownersRes.Body.Close()
		err = json.NewDecoder(ownersRes.Body).Decode(&owners)
		if err != nil {
			return nil, ErrFailedToParsePackage
		}
	}

	// In Crates.io, there's no explicit distinction between authors and maintainers
	// We'll treat all as maintainers
	maintainers := make([]Publisher, 0)
	for _, v := range owners.Users {
		maintainers = append(maintainers, Publisher{
			Name: v.Name,
			Url:  v.Url,
			ID:   v.ID,
		})
	}

	sourceRepo, err := getNormalizedGitURL(crateResp.Package.Repository)
	if err != nil {
		return nil, err
	}

	pkg := Package{
		Name:                crateResp.Package.Name,
		Description:         crateResp.Package.Description,
		SourceRepositoryUrl: sourceRepo,
		Maintainers:         maintainers,
		LatestVersion:       crateResp.Package.MaxVersion,
		Versions:            pkgVersions,
		Downloads:           OptionalInt{Value: uint64(crateResp.Package.Downloads), Valid: crateResp.Package.Downloads > 0},
		CreatedAt:           crateResp.Package.CreatedAt,
		UpdatedAt:           crateResp.Package.UpdatedAt,
	}

	return &pkg, nil
}
