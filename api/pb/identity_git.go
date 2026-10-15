package pb

import "strings"

// gitHostIdentityName is the name fold of the GitHub, GitLab and Bitbucket
// rules. Each host resolves a repository path in any case to one repository:
// Azure/login and azure/login name one GitHub Action. GitLab keeps a path
// unique without regard to case, and the PURL spec says names of the github
// and bitbucket types are not case-sensitive. Paths are ASCII, so any other
// name stays raw and matches only itself.
func gitHostIdentityName(name string) string {
	if !isASCII(name) {
		return name
	}
	return strings.ToLower(name)
}
