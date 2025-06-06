package packageregistry

import (
	"encoding/xml"
)

// Maven Central Search API response structures
// Based on: https://search.maven.org/solrsearch/select?q=g:junit&rows=20&wt=json

type mavenSearchResponse struct {
	ResponseHeader mavenResponseHeader `json:"responseHeader"`
	Response       mavenResponse       `json:"response"`
}

type mavenResponseHeader struct {
	Status int `json:"status"`
	QTime  int `json:"QTime"`
}

type mavenResponse struct {
	NumFound int        `json:"numFound"`
	Start    int        `json:"start"`
	Docs     []mavenDoc `json:"docs"`
}

type mavenDoc struct {
	Id            string   `json:"id"`
	GroupId       string   `json:"g"`
	ArtifactId    string   `json:"a"`
	LatestVersion string   `json:"latestVersion"`
	VersionCount  int      `json:"versionCount"`
	Timestamp     int64    `json:"timestamp"`
	Text          []string `json:"text"`
	RepositoryId  string   `json:"repositoryId,omitempty"`
	Packaging     string   `json:"p,omitempty"`
	EC            []string `json:"ec,omitempty"`
}

// Alternative structure for GAV (Group, Artifact, Version) core searches
type mavenGAVDoc struct {
	Id         string `json:"id"`
	GroupId    string `json:"g"`
	ArtifactId string `json:"a"`
	Version    string `json:"v"`
	Packaging  string `json:"p"`
	Timestamp  int64  `json:"timestamp"`
}

// GAV search response structure
type mavenGAVSearchResponse struct {
	ResponseHeader mavenResponseHeader `json:"responseHeader"`
	Response       mavenGAVResponse    `json:"response"`
}

type mavenGAVResponse struct {
	NumFound int           `json:"numFound"`
	Start    int           `json:"start"`
	Docs     []mavenGAVDoc `json:"docs"`
}

// POM XML parsing structures for dependency resolution
type MavenPOM struct {
	XMLName      xml.Name              `xml:"project"`
	GroupId      string                `xml:"groupId"`
	ArtifactId   string                `xml:"artifactId"`
	Version      string                `xml:"version"`
	Packaging    string                `xml:"packaging"`
	Parent       *MavenPOMParent       `xml:"parent"`
	Dependencies *MavenPOMDependencies `xml:"dependencies"`
	Properties   *MavenPOMProperties   `xml:"properties"`
}

type MavenPOMParent struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type MavenPOMDependencies struct {
	Dependencies []MavenPOMDependency `xml:"dependency"`
}

type MavenPOMDependency struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Type       string `xml:"type"`
	Optional   string `xml:"optional"`
}

type MavenPOMProperties struct {
	// We'll use a custom unmarshaler to handle arbitrary properties
	Properties map[string]string
}

// Custom XML unmarshaling for properties to handle any property name
func (p *MavenPOMProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.Properties = make(map[string]string)

	for {
		t, err := d.Token()
		if err != nil {
			return err
		}

		switch se := t.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &se); err != nil {
				return err
			}
			p.Properties[se.Name.Local] = value
		case xml.EndElement:
			if se.Name.Local == "properties" {
				return nil
			}
		}
	}
}
