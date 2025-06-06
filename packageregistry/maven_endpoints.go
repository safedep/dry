package packageregistry

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	mavenSearchRows = 100
)

// Maven Central Search API Endpoints
// Docs: https://central.sonatype.org/search/rest-api-guide/

func mavenAPIEndpointPackageURL(groupId, artifactId string) string {
	// Search for specific groupId and artifactId
	query := fmt.Sprintf("g:%s AND a:%s", groupId, artifactId)
	return fmt.Sprintf("https://search.maven.org/solrsearch/select?q=%s&rows=%d&wt=json", url.QueryEscape(query), mavenSearchRows)
}

func mavenAPIEndpointPackagesByGroupURL(groupId string) string {
	// Search for all packages in a specific groupId
	query := fmt.Sprintf("g:%s", groupId)
	return fmt.Sprintf("https://search.maven.org/solrsearch/select?q=%s&rows=%d&wt=json", url.QueryEscape(query), mavenSearchRows)
}

func mavenAPIEndpointPackageVersionsURL(groupId, artifactId string) string {
	// Search for all versions of a specific artifact
	query := fmt.Sprintf("g:%s AND a:%s", groupId, artifactId)
	return fmt.Sprintf("https://search.maven.org/solrsearch/select?q=%s&core=gav&rows=%d&wt=json", url.QueryEscape(query), mavenSearchRows)
}

// mavenAPIEndpointPomURL constructs the URL to fetch the pom.xml file for a specific package version
func mavenAPIEndpointPomURL(groupId, artifactId, version string) string {
	// Convert groupId to path format (e.g., "org.apache.commons" -> "org/apache/commons")
	groupPath := strings.ReplaceAll(groupId, ".", "/")
	return fmt.Sprintf("https://repo1.maven.org/maven2/%s/%s/%s/%s-%s.pom", groupPath, artifactId, version, artifactId, version)
}
