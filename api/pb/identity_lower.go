package pb

import "strings"

// asciiLowerIdentityName is the name fold of the registries that ignore the
// case of a name: GitHub, GitLab, Bitbucket, NuGet, Packagist and the Chrome
// Web Store. Azure/login and azure/login name one GitHub Action, and
// the PURL spec says names of the github and bitbucket types are not
// case-sensitive. Their names are ASCII, so any other name stays raw and
// matches only itself. A Unicode fold would merge a homoglyph, such as the
// KELVIN SIGN, with a real name.
func asciiLowerIdentityName(name string) string {
	if !isASCII(name) {
		return name
	}
	return strings.ToLower(name)
}
