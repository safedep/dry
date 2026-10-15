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

// gitHubActionIdentityName folds the owner and the repository of an action and
// keeps the path in the repository raw. The path names a directory in the git
// tree, which is case-sensitive: actions/cache/save and actions/cache/Save are
// two actions, and a fold would give one the verdict of the other. The frozen
// NewPurlPackageVersion lower-cases the path too, so the two differ for a
// mixed-case path. This rule is the correct one.
func gitHubActionIdentityName(name string) string {
	owner, rest, ok := strings.Cut(name, "/")
	if !ok {
		return gitHostIdentityName(name)
	}
	repo, path, hasPath := strings.Cut(rest, "/")
	repository := gitHostIdentityName(owner + "/" + repo)
	if !hasPath {
		return repository
	}
	return repository + "/" + path
}
