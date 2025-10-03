package packageregistry

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/semver"
	"github.com/stretchr/testify/assert"
)

func TestCratesGetPublisher(t *testing.T) {
	cases := []struct {
		testName   string
		pkgName    string
		pkgVersion string

		expectedError         error
		expectedPublishersMin int
		assertFunc            func(t *testing.T, publisherInfo *PackagePublisherInfo, err error)
	}{
		{
			testName:              "Correct publisher for popular Rust package",
			pkgName:               "serde",
			pkgVersion:            "1.0.225",
			expectedError:         nil,
			expectedPublishersMin: 1,
			assertFunc: func(t *testing.T, publisherInfo *PackagePublisherInfo, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.GreaterOrEqual(t, len(publisherInfo.Publishers), 1)

				// serde is owned by dtolnay and/or rust-lang team members
				hasExpectedOwner := false
				for _, publisher := range publisherInfo.Publishers {
					if publisher.Name == "David Tolnay" ||
						publisher.Name == "dtolnay" ||
						publisher.Url == "https://crates.io/users/dtolnay" {
						hasExpectedOwner = true
						break
					}
				}
				assert.True(t, hasExpectedOwner, "Expected dtolnay as an owner of serde")
			},
		},
		{
			testName:              "Correct publisher for tokio package",
			pkgName:               "tokio",
			pkgVersion:            "1.47.0",
			expectedError:         nil,
			expectedPublishersMin: 1,
			assertFunc: func(t *testing.T, publisherInfo *PackagePublisherInfo, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.GreaterOrEqual(t, len(publisherInfo.Publishers), 1)
			},
		},
		{
			testName:      "Failed to fetch non-existent package",
			pkgName:       "this-package-does-not-exist-12345678",
			pkgVersion:    "1.0.0",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewCratesAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry crates adapter: %v", err)
			}
			pd, err := adapter.PublisherDiscovery()

			if err != nil {
				t.Fatalf("failed to create publisher discovery client in crates adapter")
			}

			pkgVersion := packagev1.PackageVersion{
				Version: test.pkgVersion,
				Package: &packagev1.Package{
					Name: test.pkgName,
				},
			}

			publisherInfo, err := pd.GetPackagePublisher(&pkgVersion)

			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else if test.assertFunc != nil {
				test.assertFunc(t, publisherInfo, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.GreaterOrEqual(t, len(publisherInfo.Publishers), test.expectedPublishersMin)
			}
		})
	}
}

func TestCratesGetPackagesByPublisher(t *testing.T) {
	cases := []struct {
		testName    string
		publisherID int

		expectedError       error
		expectedMinPackages int
		expectedPkgName     string
	}{
		{
			testName:            "Get packages by 361 ID",
			publisherID:         361,
			expectedError:       nil,
			expectedMinPackages: 1,
			expectedPkgName:     "cereal",
		},
		{
			testName:      "Non-existent publisher",
			publisherID:   000000000,
			expectedError: ErrAuthorNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewCratesAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry crates adapter: %v", err)
			}

			pd, err := adapter.PublisherDiscovery()
			if err != nil {
				t.Fatalf("failed to create publisher discovery client in crates adapter")
			}

			pkgs, err := pd.GetPublisherPackages(Publisher{ID: test.publisherID})
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.expectedMinPackages)

				// Check if the expected package is in the results
				if test.expectedPkgName != "" {
					found := false
					for _, pkg := range pkgs {
						if pkg.Name == test.expectedPkgName {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected package %s not found in results", test.expectedPkgName)
				}
			}
		})
	}
}

func TestCratesPackageDiscoveryDownloadStats(t *testing.T) {
	cases := []struct {
		pkgName                   string
		expectedError             error
		expectedMinTotalDownloads uint64
	}{
		{
			pkgName:                   "serde",
			expectedError:             nil,
			expectedMinTotalDownloads: 100_000_000, // serde is extremely popular
		},
		{
			pkgName:                   "tokio",
			expectedError:             nil,
			expectedMinTotalDownloads: 50_000_000,
		},
		{
			pkgName:       "this-package-does-not-exist-12345678",
			expectedError: ErrPackageNotFound,
		},
	}

	adapter, _ := NewCratesAdapter()
	packageDiscovery, err := adapter.PackageDiscovery()
	assert.NoError(t, err)

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			downloadStats, err := packageDiscovery.GetPackageDownloadStats(test.pkgName)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, downloadStats)

				// Check total downloads
				assert.GreaterOrEqual(t, downloadStats.Total, test.expectedMinTotalDownloads)
			}
		})
	}
}

func TestCratesGetPackage(t *testing.T) {
	cases := []struct {
		pkgName string

		expectedError        error
		expectedMinDownloads uint64
		expectedRepoURL      string
		expectedMinVersions  int
	}{
		{
			pkgName:              "serde",
			expectedError:        nil,
			expectedMinDownloads: 100_000_000,
			expectedRepoURL:      "https://github.com/serde-rs/serde",
			expectedMinVersions:  50, // serde has many versions
		},
		{
			pkgName:              "tokio",
			expectedError:        nil,
			expectedMinDownloads: 50_000_000,
			expectedRepoURL:      "https://github.com/tokio-rs/tokio",
			expectedMinVersions:  30,
		},
		{
			pkgName:       "this-package-does-not-exist-12345678",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewCratesAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry crates adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in crates adapter")
			}

			pkg, err := pd.GetPackage(test.pkgName)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)

				// Downloads data
				assert.True(t, pkg.Downloads.Valid)
				assert.GreaterOrEqual(t, pkg.Downloads.Value, test.expectedMinDownloads)

				// Repository data
				assert.Equal(t, test.expectedRepoURL, pkg.SourceRepositoryUrl)

				// Version counts
				assert.GreaterOrEqual(t, len(pkg.Versions), test.expectedMinVersions)

				// Latest version should be set
				assert.NotEmpty(t, pkg.LatestVersion)
			}
		})
	}
}

func TestCratesGetPackageLatestVersion(t *testing.T) {
	cases := []struct {
		pkgName            string
		expectedError      error
		expectedMinVersion string
	}{
		{
			pkgName:            "serde",
			expectedError:      nil,
			expectedMinVersion: "1.0.225", // We expect it to be at least this version
		},
		{
			pkgName:            "tokio",
			expectedError:      nil,
			expectedMinVersion: "1.47.0",
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewCratesAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry crates adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in crates adapter")
			}

			pkg, err := pd.GetPackage(test.pkgName)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.True(t, semver.IsAheadOrEqual(test.expectedMinVersion, pkg.LatestVersion),
					"Expected %s to be at least version %s, got %s", test.pkgName, test.expectedMinVersion, pkg.LatestVersion)
			}
		})
	}
}

func TestCratesGetPackageDependencies(t *testing.T) {
	cases := []struct {
		name       string
		pkgName    string
		pkgVersion string
		assertFn   func(t *testing.T, dependencies *PackageDependencyList, err error)
	}{
		{
			name:       "serde dependencies",
			pkgName:    "serde",
			pkgVersion: "1.0.225",
			assertFn: func(t *testing.T, dependencies *PackageDependencyList, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, dependencies)

				assert.Contains(t, dependencies.Dependencies, PackageDependencyInfo{
					Name:        "serde_derive",
					VersionSpec: "^1",
				})

				assert.Contains(t, dependencies.Dependencies, PackageDependencyInfo{
					Name:        "serde_core",
					VersionSpec: "=1.0.225",
				})
			},
		},
		{
			name:       "tokio dependencies",
			pkgName:    "tokio",
			pkgVersion: "1.47.0",
			assertFn: func(t *testing.T, dependencies *PackageDependencyList, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, dependencies)

				assert.Contains(t, dependencies.Dependencies, PackageDependencyInfo{
					Name:        "backtrace",
					VersionSpec: "^0.3.58",
				})

				// Verify minimum dependency counts
				assert.GreaterOrEqual(t, len(dependencies.Dependencies), 5, "Expected at least 5 dependencies")
				assert.GreaterOrEqual(t, len(dependencies.DevDependencies), 3, "Expected at least 3 dev dependencies")
			},
		},
		{
			name:       "invalid package",
			pkgName:    "this-package-does-not-exist-12345678",
			pkgVersion: "1.0.0",
			assertFn: func(t *testing.T, dependencies *PackageDependencyList, err error) {
				assert.ErrorIs(t, err, ErrPackageNotFound)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewCratesAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry crates adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in crates adapter")
			}

			dependencies, err := pd.GetPackageDependencies(test.pkgName, test.pkgVersion)
			test.assertFn(t, dependencies, err)
		})
	}
}
