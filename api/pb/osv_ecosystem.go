package pb

import (
	"fmt"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// osvEcosystemNames holds the name of each ecosystem in the OSV schema
// (https://ossf.github.io/osv-schema/#affectedpackage-field). OSV owns these
// names, and stored OSV data and OSV queries hold them. A wrong name loses
// every record of the ecosystem with no error, so add names and never change
// one. An ecosystem that OSV does not publish has no entry.
//
// OSV names a registry that shares the format of another ecosystem with a
// suffix: the OpenVSX registry serves VS Code extensions, so its name is
// VSCode with the registry URL. The OSV data dump keeps such records under
// the name before the colon.
var osvEcosystemNames = map[packagev1.Ecosystem]string{
	packagev1.Ecosystem_ECOSYSTEM_MAVEN:          "Maven",
	packagev1.Ecosystem_ECOSYSTEM_NPM:            "npm",
	packagev1.Ecosystem_ECOSYSTEM_PYPI:           "PyPI",
	packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:       "RubyGems",
	packagev1.Ecosystem_ECOSYSTEM_NUGET:          "NuGet",
	packagev1.Ecosystem_ECOSYSTEM_CARGO:          "crates.io",
	packagev1.Ecosystem_ECOSYSTEM_GO:             "Go",
	packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS: "GitHub Actions",
	packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:      "Packagist",
	packagev1.Ecosystem_ECOSYSTEM_VSCODE:         "VSCode",
	packagev1.Ecosystem_ECOSYSTEM_PUB:            "Pub",
	packagev1.Ecosystem_ECOSYSTEM_OPENVSX:        "VSCode:https://open-vsx.org",
}

var ecosystemsByOSVName = func() map[string]packagev1.Ecosystem {
	byName := make(map[string]packagev1.Ecosystem, len(osvEcosystemNames))
	for ecosystem, name := range osvEcosystemNames {
		byName[name] = ecosystem
	}
	return byName
}()

// OSVEcosystemName returns the OSV name of an ecosystem, such as "crates.io"
// for ECOSYSTEM_CARGO, or "VSCode:https://open-vsx.org" for
// ECOSYSTEM_OPENVSX. It returns an error when OSV has no such ecosystem.
func OSVEcosystemName(ecosystem packagev1.Ecosystem) (string, error) {
	name, ok := osvEcosystemNames[ecosystem]
	if !ok {
		return "", fmt.Errorf("ecosystem %s has no OSV ecosystem", ecosystem)
	}
	return name, nil
}

// EcosystemFromOSVName returns the ecosystem of an OSV name. The match is
// exact, unlike EcosystemFromName: OSV names are case-sensitive, and this
// function reads names from OSV data, not names that a person types. The
// suffix is part of the name: "VSCode:https://open-vsx.org" is OpenVSX, and
// a name with any other suffix, such as "Debian:12", returns an error.
func EcosystemFromOSVName(name string) (packagev1.Ecosystem, error) {
	ecosystem, ok := ecosystemsByOSVName[name]
	if !ok {
		return packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, fmt.Errorf("unknown OSV ecosystem name: %q", name)
	}
	return ecosystem, nil
}

// OSVEcosystems returns every ecosystem that has an OSV name, in the order of
// the enum values. A tool that reads the OSV data dump one ecosystem at a time
// must use the name before the colon as the directory, and read a directory
// once: OpenVSX records are in the VSCode directory.
func OSVEcosystems() []packagev1.Ecosystem {
	ecosystems := make([]packagev1.Ecosystem, 0, len(osvEcosystemNames))
	for value := range len(packagev1.Ecosystem_name) {
		if _, ok := osvEcosystemNames[packagev1.Ecosystem(value)]; ok {
			ecosystems = append(ecosystems, packagev1.Ecosystem(value))
		}
	}
	return ecosystems
}
