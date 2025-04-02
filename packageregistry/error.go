package packageregistry

import (
	"errors"
)

var (
	ErrPackageNotFound         = errors.New("package not found")
	ErrFailedToFetchPackage    = errors.New("failed to fetch package")
	ErrFailedToParsePackage    = errors.New("failed to parse package")
	ErrFailedToParseNpmPackage = errors.New("failed to parse npm package")
)
