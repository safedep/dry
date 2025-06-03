package packageregistry

import (
	"github.com/safedep/dry/semver"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGoGetPackage(t *testing.T) {
	cases := []struct {
		pkgName string

		expectedError            error
		expectedRepoURL          string
		expectedMinVersionCounts int
		expectedBaseVersion      string
	}{
		{
			pkgName:                  "github.com/gin-gonic/gin",
			expectedError:            nil,
			expectedRepoURL:          "https://github.com/gin-gonic/gin",
			expectedMinVersionCounts: 25,
			expectedBaseVersion:      "v1.10.1",
		},
		{
			pkgName:                  "golang.org/x/mod",
			expectedError:            nil,
			expectedRepoURL:          "https://go.googlesource.com/mod",
			expectedMinVersionCounts: 28,
			expectedBaseVersion:      "v0.24.0",
		},

		{
			pkgName:       "random-package-name-that-does-not-exist-1246890",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewGoAdapter()

			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in npm adapter")
			}

			pkg, err := pd.GetPackage(test.pkgName)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)
				// Repository data
				assert.Equal(t, test.expectedRepoURL, pkg.SourceRepositoryUrl)

				assert.GreaterOrEqual(t, len(pkg.Versions), test.expectedMinVersionCounts)

				assert.True(t, semver.IsAheadOrEqual(test.expectedBaseVersion, pkg.LatestVersion))
			}
		})
	}
}

func TestGoGetDependencies(t *testing.T) {
	cases := []struct {
		pkgName    string
		pkgVersion string

		expectedError        error
		expectedDepsCount    int
		expectedDevDepsCount int
	}{
		{
			pkgName:           "github.com/ollama/ollama",
			pkgVersion:        "v0.9.0",
			expectedError:     nil,
			expectedDepsCount: 66,
		},
		{
			pkgName:    "go.uber.org/zap",
			pkgVersion: "v1.27.0",

			expectedError:     nil,
			expectedDepsCount: 7,
		},
		{
			pkgName:    "github.com/safedep/vet",
			pkgVersion: "v1.11.0",

			expectedError:     nil,
			expectedDepsCount: 428,
		},
		{
			pkgName:       "random-package-name-that-does-not-exist-1246890",
			pkgVersion:    "v1.0.0",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewGoAdapter()

			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in npm adapter")
			}

			pkg, err := pd.GetPackageDependencies(test.pkgName, test.pkgVersion)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)

				assert.Equal(t, len(pkg.Dependencies), test.expectedDepsCount)
			}
		})
	}
}
