package packageregistry

import (
	"reflect"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestGithubPackageRegistryAdapter_GetPackage(t *testing.T) {
	cases := []struct {
		packageName string

		expectedErr           error
		expectedDescription   bool
		expectedSourceURL     string
		expectedAuthorName    string
		expectedAuthorEmail   string
		expectedLatestVersion string
		expectedMinVersions   int
	}{
		{
			packageName: "safedep/vet",

			expectedErr:           nil,
			expectedDescription:   true,
			expectedSourceURL:     "https://github.com/safedep/vet",
			expectedAuthorName:    "safedep",
			expectedAuthorEmail:   "",
			expectedLatestVersion: "v1.9.9", // we will do >=
			expectedMinVersions:   10,       // vet has minimum 10 releases (versions)
		},
		{
			// Good test where there is no release
			packageName: "safedep/dry",

			expectedErr:           nil,
			expectedDescription:   true,
			expectedSourceURL:     "https://github.com/safedep/dry",
			expectedAuthorName:    "safedep",
			expectedAuthorEmail:   "",
			expectedLatestVersion: "main", // default branch, since no releases are there in this repo
			expectedMinVersions:   0,      // dry has no releases
		},
		{
			packageName: "KunalSin9h/livejq",

			expectedErr:           nil,
			expectedDescription:   true,
			expectedSourceURL:     "https://github.com/KunalSin9h/livejq",
			expectedAuthorName:    "KunalSin9h",
			expectedAuthorEmail:   "kunal@kunalsin9h.com",
			expectedLatestVersion: "v2.0.0", // we will do >=
			expectedMinVersions:   2,        // livejq has minimum 2 releases (versions)
		},
		{
			packageName: "somerandomuser/non-existing-package",

			expectedErr: ErrNoPackagesFound,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.packageName, func(t *testing.T) {
			t.Parallel()

			gha, err := NewGithubPackageRegistryAdapter()
			if err != nil {
				t.Fatalf("Failed to create github package registry adapter: %v", err)
			}

			pd, err := gha.PackageDiscovery()
			if err != nil {
				t.Fatalf("Failed to get package: %v", err)
			}

			pkg, err := pd.GetPackage(testCase.packageName)
			if testCase.expectedErr != nil {
				assert.Error(t, err)
				assert.Nil(t, pkg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)

				assert.Equal(t, testCase.packageName, pkg.Name)
				assert.Equal(t, testCase.expectedDescription, pkg.Description != "")
				assert.Equal(t, testCase.expectedSourceURL, pkg.SourceRepositoryUrl)
				assert.Equal(t, testCase.expectedAuthorName, pkg.Author.Name)
				assert.Equal(t, testCase.expectedAuthorEmail, pkg.Author.Email)

				assert.GreaterOrEqual(t, len(pkg.Versions), testCase.expectedMinVersions)
				assert.GreaterOrEqual(t, pkg.LatestVersion, testCase.expectedLatestVersion)
			}
		})
	}
}

func TestGithubPackageRegistryAdapter_GetPublisherPackages(t *testing.T) {
	cases := []struct {
		publisherName    string
		expectedErr      error
		expectedPackages int
	}{
		{
			publisherName: "safedep",

			expectedErr:      nil,
			expectedPackages: 30, // more then 10 repos
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.publisherName, func(t *testing.T) {
			t.Parallel()

			gha, err := NewGithubPackageRegistryAdapter()
			if err != nil {
				t.Fatalf("Failed to create github package registry adapter: %v", err)
			}

			pd, err := gha.PublisherDiscovery()
			if err != nil {
				t.Fatalf("Failed to get package: %v", err)
			}

			pkgs, err := pd.GetPublisherPackages(Publisher{Name: testCase.publisherName})
			if testCase.expectedErr != nil {
				assert.Error(t, err)
				assert.Nil(t, pkgs)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)

				assert.GreaterOrEqual(t, len(pkgs), testCase.expectedPackages)
			}
		})
	}
}

func TestGithubPackageRegistryAdapter_GetPackagePublisher(t *testing.T) {
	cases := []struct {
		packageName       string
		expectedErr       error
		expectedPublisher Publisher
	}{
		{
			packageName: "safedep/vet",

			expectedErr:       nil,
			expectedPublisher: Publisher{Name: "safedep", Email: "", Url: "https://github.com/safedep"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.packageName, func(t *testing.T) {
			t.Parallel()

			gha, err := NewGithubPackageRegistryAdapter()
			if err != nil {
				t.Fatalf("Failed to create github package registry adapter: %v", err)
			}

			pd, err := gha.PublisherDiscovery()
			if err != nil {
				t.Fatalf("Failed to get package: %v", err)
			}

			pkgVersion := &packagev1.PackageVersion{
				Package: &packagev1.Package{
					Name: testCase.packageName,
				},
			}

			pkg, err := pd.GetPackagePublisher(pkgVersion)
			if testCase.expectedErr != nil {
				assert.Error(t, err)
				assert.Nil(t, pkg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)

				if !reflect.DeepEqual(testCase.expectedPublisher, pkg.Publishers[0]) {
					t.Errorf("expected: %v, got: %v", testCase.expectedPublisher, pkg.Publishers[0])
				}
			}
		})
	}
}
