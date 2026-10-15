package pb

import "strings"

// gitHostIdentityName is the name fold of the GitHub and Bitbucket rules. Both
// hosts resolve an owner and a repository name in any case to one repository:
// Azure/login and azure/login name one GitHub Action. The PURL spec says the
// same for the github and bitbucket types. Owner and repository names are
// ASCII, so any other name stays raw and matches only itself.
func gitHostIdentityName(name string) string {
	if !isASCII(name) {
		return name
	}
	return strings.ToLower(name)
}
