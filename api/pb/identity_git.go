package pb

import "strings"

// gitHubActionIdentityName folds the owner and the repository of an action and
// keeps the path in the repository raw. The path names a directory in the git
// tree, which is case-sensitive: actions/cache/save and actions/cache/Save are
// two actions, and a fold would give one the verdict of the other. The frozen
// NewPurlPackageVersion lower-cases the path too, so the two differ for a
// mixed-case path. This rule is the correct one.
func gitHubActionIdentityName(name string) string {
	owner, rest, ok := strings.Cut(name, "/")
	if !ok {
		return asciiLowerIdentityName(name)
	}
	repo, path, hasPath := strings.Cut(rest, "/")
	repository := asciiLowerIdentityName(owner + "/" + repo)
	if !hasPath {
		return repository
	}
	return repository + "/" + path
}
