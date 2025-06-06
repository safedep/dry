package packageregistry

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

type mavenAdapter struct{}
type mavenPublisherDiscovery struct{}
type mavenPackageDiscovery struct{}

// Verify that mavenAdapter implements the Client interface
var _ Client = (*mavenAdapter)(nil)

// parseMavenCoordinates parses a Maven package name in the format "groupId:artifactId"
// and returns the groupId and artifactId components, or an error if the format is invalid
func parseMavenCoordinates(packageName string) (groupId, artifactId string, err error) {
	parts := strings.Split(packageName, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid Maven package name format: %s (expected groupId:artifactId)", packageName)
	}
	return parts[0], parts[1], nil
}

// NewMavenAdapter creates a new Maven registry adapter
func NewMavenAdapter() (Client, error) {
	return &mavenAdapter{}, nil
}

func (ma *mavenAdapter) PublisherDiscovery() (PublisherDiscovery, error) {
	return &mavenPublisherDiscovery{}, nil
}

func (ma *mavenAdapter) PackageDiscovery() (PackageDiscovery, error) {
	return &mavenPackageDiscovery{}, nil
}

// GetPackagePublisher returns the publisher of a Maven package
func (mp *mavenPublisherDiscovery) GetPackagePublisher(packageVersion *packagev1.PackageVersion) (*PackagePublisherInfo, error) {
	packageName := packageVersion.GetPackage().GetName()

	// For Maven, we need to parse the package name to get groupId and artifactId
	groupId, artifactId, err := parseMavenCoordinates(packageName)
	if err != nil {
		return nil, err
	}

	// Get package details
	searchResult, err := mavenGetPackageSearchResult(groupId, artifactId)
	if err != nil {
		return nil, fmt.Errorf("failed to get package details: %w", err)
	}

	// Check if package exists
	if len(searchResult.Response.Docs) == 0 {
		return nil, ErrPackageNotFound
	}

	// Extract publisher information if available
	publishers := make([]Publisher, 0)
	// For Maven, we don't have detailed publisher info from the search API
	// We'll use the groupId as the publisher name
	publishers = append(publishers, Publisher{
		Name:  groupId,
		Email: "", // Not available in Maven Central search API
		Url:   "", // Not available in Maven Central search API
	})

	return &PackagePublisherInfo{Publishers: publishers}, nil
}

// GetPublisherPackages returns all packages published by a given publisher
func (mp *mavenPublisherDiscovery) GetPublisherPackages(publisher Publisher) ([]*Package, error) {
	// Search for packages by groupId (using publisher name as groupId)
	url := mavenAPIEndpointPackagesByGroupURL(publisher.Name)

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

	var searchResult mavenSearchResponse
	err = json.NewDecoder(res.Body).Decode(&searchResult)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	if len(searchResult.Response.Docs) == 0 {
		return nil, ErrNoPackagesFound
	}

	packages := make([]*Package, 0)
	for _, doc := range searchResult.Response.Docs {
		pkg, err := convertMavenDocToPackage(doc)
		if err != nil {
			continue // Skip packages with conversion errors
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

func (mp *mavenPackageDiscovery) GetPackage(packageName string) (*Package, error) {
	// Parse Maven package name (groupId:artifactId)
	groupId, artifactId, err := parseMavenCoordinates(packageName)
	if err != nil {
		return nil, err
	}

	return mavenGetPackageDetails(groupId, artifactId)
}

func (mp *mavenPackageDiscovery) GetPackageDependencies(packageName string, packageVersion string) (*PackageDependencyList, error) {
	// Parse Maven package name (groupId:artifactId)
	groupId, artifactId, err := parseMavenCoordinates(packageName)
	if err != nil {
		return nil, err
	}

	// Fetch and parse the pom.xml file
	pom, err := mavenFetchAndParsePOM(groupId, artifactId, packageVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch or parse POM: %w", err)
	}

	dependencies := make([]PackageDependencyInfo, 0)
	devDependencies := make([]PackageDependencyInfo, 0)

	if pom.Dependencies != nil {
		for _, dep := range pom.Dependencies.Dependencies {
			// Resolve version if it contains properties
			version := mavenResolveProperty(dep.Version, pom)

			// Skip dependencies with missing information
			if dep.GroupId == "" || dep.ArtifactId == "" || version == "" {
				continue
			}

			dependencyInfo := PackageDependencyInfo{
				Name:        fmt.Sprintf("%s:%s", dep.GroupId, dep.ArtifactId),
				VersionSpec: version,
			}

			// Classify dependencies based on scope
			switch dep.Scope {
			case "test":
				devDependencies = append(devDependencies, dependencyInfo)
			case "provided", "runtime", "compile", "":
				// Default scope is compile, so empty scope is treated as runtime dependency
				dependencies = append(dependencies, dependencyInfo)
			}
		}
	}

	return &PackageDependencyList{
		Dependencies:    dependencies,
		DevDependencies: devDependencies,
	}, nil
}

func (mp *mavenPackageDiscovery) GetPackageDownloadStats(packageName string) (DownloadStats, error) {
	// Maven Central doesn't provide download statistics through their public API
	return DownloadStats{}, fmt.Errorf("download stats are not available for Maven Central")
}

func mavenGetPackageSearchResult(groupId, artifactId string) (*mavenSearchResponse, error) {
	url := mavenAPIEndpointPackageURL(groupId, artifactId)

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

	var searchResult mavenSearchResponse
	err = json.NewDecoder(res.Body).Decode(&searchResult)
	if err != nil {
		return nil, ErrFailedToParsePackage
	}

	return &searchResult, nil
}

func mavenGetPackageDetails(groupId, artifactId string) (*Package, error) {
	searchResult, err := mavenGetPackageSearchResult(groupId, artifactId)
	if err != nil {
		return nil, err
	}

	if len(searchResult.Response.Docs) == 0 {
		return nil, ErrPackageNotFound
	}

	// Get the most recent version (first in the list)
	doc := searchResult.Response.Docs[0]
	return convertMavenDocToPackage(doc)
}

func convertMavenDocToPackage(doc mavenDoc) (*Package, error) {
	// Get all versions for this package using GAV core search
	versions := make([]PackageVersionInfo, 0)

	// Fetch all versions using the GAV core
	gavVersions, err := mavenGetAllVersions(doc.GroupId, doc.ArtifactId)
	if err != nil {
		// If we can't get all versions, at least include the latest
		if doc.LatestVersion != "" {
			versions = append(versions, PackageVersionInfo{
				Version: doc.LatestVersion,
			})
		}
	} else {
		versions = gavVersions
	}

	// Determine source repository URL - Maven Central doesn't provide this in basic search
	sourceRepoURL := ""

	// Create publisher from groupId
	author := Publisher{
		Name:  doc.GroupId,
		Email: "", // Not available
		Url:   "", // Not available
	}

	pkg := Package{
		Name:                fmt.Sprintf("%s:%s", doc.GroupId, doc.ArtifactId),
		Description:         "", // Not available in basic search response
		SourceRepositoryUrl: sourceRepoURL,
		Author:              author,
		Maintainers:         []Publisher{author}, // Same as author for Maven
		LatestVersion:       doc.LatestVersion,
		Versions:            versions,
		Downloads: OptionalInt{
			Valid: false, // Maven Central doesn't provide download stats
			Value: 0,
		},
		CreatedAt: time.Time{}, // Not available in search API
		UpdatedAt: mavenParseTimestamp(doc.Timestamp),
	}

	return &pkg, nil
}

func mavenGetAllVersions(groupId, artifactId string) ([]PackageVersionInfo, error) {
	url := mavenAPIEndpointPackageVersionsURL(groupId, artifactId)

	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch versions")
	}

	var gavResponse mavenGAVSearchResponse
	err = json.NewDecoder(res.Body).Decode(&gavResponse)
	if err != nil {
		return nil, err
	}

	versions := make([]PackageVersionInfo, 0)
	for _, gavDoc := range gavResponse.Response.Docs {
		versions = append(versions, PackageVersionInfo{
			Version: gavDoc.Version,
		})
	}

	return versions, nil
}

func mavenParseTimestamp(timestamp int64) time.Time {
	if timestamp == 0 {
		return time.Time{}
	}
	return time.UnixMilli(timestamp)
}

// mavenFetchAndParsePOM fetches the pom.xml file for a given package version and parses it
func mavenFetchAndParsePOM(groupId, artifactId, version string) (*mavenPOM, error) {
	url := mavenAPIEndpointPomURL(groupId, artifactId, version)

	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch POM file: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("POM file not found for %s:%s:%s", groupId, artifactId, version)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch POM file, status: %d", res.StatusCode)
	}

	var pom mavenPOM
	err = xml.NewDecoder(res.Body).Decode(&pom)
	if err != nil {
		return nil, fmt.Errorf("failed to parse POM XML: %w", err)
	}

	return &pom, nil
}

// mavenResolveProperty resolves Maven properties in version strings
// This is a simplified implementation that handles basic property resolution
func mavenResolveProperty(version string, pom *mavenPOM) string {
	if version == "" {
		return ""
	}

	// Handle simple property references like ${property.name}
	if strings.HasPrefix(version, "${") && strings.HasSuffix(version, "}") {
		propertyName := version[2 : len(version)-1]

		// Check for common built-in properties
		switch propertyName {
		case "project.version":
			if pom.Version != "" {
				return pom.Version
			}
			// If project version is not set, check parent version
			if pom.Parent != nil && pom.Parent.Version != "" {
				return pom.Parent.Version
			}
		case "project.groupId":
			if pom.GroupId != "" {
				return pom.GroupId
			}
			if pom.Parent != nil && pom.Parent.GroupId != "" {
				return pom.Parent.GroupId
			}
		}

		// Check custom properties from the properties section
		if pom.Properties != nil && pom.Properties.Properties != nil {
			if value, exists := pom.Properties.Properties[propertyName]; exists {
				return value
			}
		}

		// Return the original version if we can't resolve
		return version
	}

	return version
}
