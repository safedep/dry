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

// nugetIdentityName folds the ASCII letters of a NuGet id and keeps every
// other character as it is. NuGet ids can hold non-ASCII letters, and a fold
// of the whole name only when it is ASCII would keep "Microsoft.Évil" raw, so
// a block on "pkg:nuget/Microsoft.*" would not select it. NuGet compares ids
// with OrdinalIgnoreCase, so this fold merges only ids that NuGet merges too.
// A Unicode fold would merge the KELVIN SIGN with "k", which NuGet keeps apart.
func nugetIdentityName(name string) string {
	// A byte loop, not strings.Map: strings.Map turns each invalid UTF-8 byte
	// into U+FFFD, and so merges two ids that differ in such a byte.
	b := []byte(name)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
