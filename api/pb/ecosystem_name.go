package pb

import (
	"fmt"
	"strings"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// ecosystemNames holds the one name of each ecosystem that a person reads and
// writes: in a flag, a config file, a report or a policy. Every SafeDep tool
// uses this table, so a user sees one name for one ecosystem in every tool.
// A name is a contract. A rename breaks each config and policy that holds the
// old name, so add names and never change one.
var ecosystemNames = map[packagev1.Ecosystem]string{
	packagev1.Ecosystem_ECOSYSTEM_MAVEN:                "maven",
	packagev1.Ecosystem_ECOSYSTEM_NPM:                  "npm",
	packagev1.Ecosystem_ECOSYSTEM_PYPI:                 "pypi",
	packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:             "rubygems",
	packagev1.Ecosystem_ECOSYSTEM_NUGET:                "nuget",
	packagev1.Ecosystem_ECOSYSTEM_CARGO:                "cargo",
	packagev1.Ecosystem_ECOSYSTEM_GO:                   "go",
	packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS:       "github-actions",
	packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:            "packagist",
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM:            "terraform",
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_MODULE:     "terraform-module",
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_PROVIDER:   "terraform-provider",
	packagev1.Ecosystem_ECOSYSTEM_VSCODE:               "vscode",
	packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY:    "github-repository",
	packagev1.Ecosystem_ECOSYSTEM_OPENVSX:              "openvsx",
	packagev1.Ecosystem_ECOSYSTEM_HOMEBREW:             "homebrew",
	packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY:    "gitlab-repository",
	packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY: "bitbucket-repository",
	packagev1.Ecosystem_ECOSYSTEM_PUB:                  "pub",
}

var ecosystemsByName = func() map[string]packagev1.Ecosystem {
	byName := make(map[string]packagev1.Ecosystem, len(ecosystemNames))
	for ecosystem, name := range ecosystemNames {
		byName[name] = ecosystem
	}
	return byName
}()

// EcosystemName returns the name of an ecosystem. It returns an error for
// ECOSYSTEM_UNSPECIFIED and for a value with no name.
func EcosystemName(ecosystem packagev1.Ecosystem) (string, error) {
	name, ok := ecosystemNames[ecosystem]
	if !ok {
		return "", fmt.Errorf("ecosystem %s has no name", ecosystem)
	}
	return name, nil
}

// EcosystemFromName returns the ecosystem of a name. The match ignores case,
// so a person can write PyPI or pypi.
func EcosystemFromName(name string) (packagev1.Ecosystem, error) {
	ecosystem, ok := ecosystemsByName[strings.ToLower(name)]
	if !ok {
		return packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, fmt.Errorf("unknown ecosystem name: %q", name)
	}
	return ecosystem, nil
}

// EcosystemNames returns every ecosystem name, in the order of the enum
// values. A tool uses it for help text and for a schema enum.
func EcosystemNames() []string {
	names := make([]string, 0, len(ecosystemNames))
	for value := range len(packagev1.Ecosystem_name) {
		if name, ok := ecosystemNames[packagev1.Ecosystem(value)]; ok {
			names = append(names, name)
		}
	}
	return names
}
