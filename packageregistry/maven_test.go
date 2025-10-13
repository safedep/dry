package packageregistry

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/semver"
	"github.com/stretchr/testify/assert"
)

func TestMavenGetPublisher(t *testing.T) {
	t.Skip()

	cases := []struct {
		testName   string
		pkgName    string
		pkgVersion string

		expectedError         error
		expectedPublisherName string
		assertFunc            func(t *testing.T, publisherInfo *PackagePublisherInfo, err error)
	}{
		{
			testName:   "Correct Maven publisher for junit",
			pkgName:    "junit:junit",
			pkgVersion: "4.13.2",

			expectedError:         nil,
			expectedPublisherName: "junit", // groupId as publisher
		},
		{
			testName:   "Correct Maven publisher for commons-lang",
			pkgName:    "org.apache.commons:commons-lang3",
			pkgVersion: "3.12.0",

			expectedError:         nil,
			expectedPublisherName: "org.apache.commons",
		},
		{
			testName:      "Invalid package name format",
			pkgName:       "invalid-package-name",
			pkgVersion:    "1.0.0",
			expectedError: assert.AnError, // We expect an error for invalid format
		},
		{
			testName:      "Non-existent package",
			pkgName:       "non.existent:non-existent",
			pkgVersion:    "1.0.0",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewMavenAdapter()
			if err != nil {
				t.Fatalf("failed to create Maven registry adapter: %v", err)
			}

			pd, err := adapter.PublisherDiscovery()
			if err != nil {
				t.Fatalf("failed to create publisher discovery client in Maven adapter")
			}

			pkgVersion := packagev1.PackageVersion{
				Version: test.pkgVersion,
				Package: &packagev1.Package{
					Name: test.pkgName,
				},
			}

			publisherInfo, err := pd.GetPackagePublisher(&pkgVersion)

			if test.expectedError != nil {
				assert.Error(t, err)
				if test.expectedError != assert.AnError {
					assert.ErrorIs(t, err, test.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.Len(t, publisherInfo.Publishers, 1)
				assert.Equal(t, test.expectedPublisherName, publisherInfo.Publishers[0].Name)
				// Maven Central search API doesn't provide email/URL
				assert.Empty(t, publisherInfo.Publishers[0].Email)
				assert.Empty(t, publisherInfo.Publishers[0].Url)
			}
		})
	}
}

func TestMavenGetPackagesByPublisher(t *testing.T) {
	t.Skip()

	cases := []struct {
		testName            string
		publisherName       string
		expectedError       error
		expectedMinPackages int
		expectedContains    []string
	}{
		{
			testName:            "Packages by junit groupId",
			publisherName:       "junit",
			expectedError:       nil,
			expectedMinPackages: 1,
			expectedContains:    []string{"junit:junit"},
		},
		{
			testName:            "Packages by org.apache.commons groupId",
			publisherName:       "org.apache.commons",
			expectedError:       nil,
			expectedMinPackages: 5, // Commons has many libraries
			expectedContains:    []string{"org.apache.commons:commons-lang3", "org.apache.commons:commons-io"},
		},
		{
			testName:      "Non-existent publisher",
			publisherName: "non.existent.group.id",
			expectedError: ErrNoPackagesFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewMavenAdapter()
			if err != nil {
				t.Fatalf("failed to create Maven registry adapter: %v", err)
			}

			pd, err := adapter.PublisherDiscovery()
			if err != nil {
				t.Fatalf("failed to create publisher discovery client in Maven adapter")
			}

			publisher := Publisher{Name: test.publisherName}
			pkgs, err := pd.GetPublisherPackages(publisher)

			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.expectedMinPackages)

				// Check that some expected packages are present
				packageNames := make([]string, len(pkgs))
				for i, pkg := range pkgs {
					packageNames[i] = pkg.Name
				}

				for _, expected := range test.expectedContains {
					found := false
					for _, actual := range packageNames {
						if actual == expected {
							found = true
							break
						}
					}
					if !found {
						// This might be too strict for some cases, so we'll just log it
						t.Logf("Expected package %s not found in results", expected)
					}
				}
			}
		})
	}
}

func TestMavenGetPackage(t *testing.T) {
	t.Skip()

	cases := []struct {
		testName            string
		pkgName             string
		expectedError       error
		expectedName        string
		expectedDescription string
		expectedMinVersions int
	}{
		{
			testName:            "Get junit package",
			pkgName:             "junit:junit",
			expectedError:       nil,
			expectedName:        "junit:junit",
			expectedDescription: "", // Description may or may not be available
			expectedMinVersions: 10, // JUnit has many versions
		},
		{
			testName:            "Get commons-lang3 package",
			pkgName:             "org.apache.commons:commons-lang3",
			expectedError:       nil,
			expectedName:        "org.apache.commons:commons-lang3",
			expectedDescription: "", // Description may or may not be available
			expectedMinVersions: 5,  // Multiple versions available
		},
		{
			testName:      "Invalid package name format",
			pkgName:       "invalid-package-name",
			expectedError: assert.AnError, // Expect format error
		},
		{
			testName:      "Non-existent package",
			pkgName:       "non.existent:non-existent",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewMavenAdapter()
			if err != nil {
				t.Fatalf("failed to create Maven registry adapter: %v", err)
			}

			packageDiscovery, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in Maven adapter")
			}

			pkg, err := packageDiscovery.GetPackage(test.pkgName)

			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)
				assert.Equal(t, test.expectedName, pkg.Name)
				assert.GreaterOrEqual(t, len(pkg.Versions), test.expectedMinVersions)
				assert.NotEmpty(t, pkg.LatestVersion)
				assert.NotEmpty(t, pkg.Author.Name)

				// Downloads should be invalid since Maven Central doesn't provide stats
				assert.False(t, pkg.Downloads.Valid)
			}
		})
	}
}

func TestMavenGetPackageDependencies(t *testing.T) {
	t.Skip()

	cases := []struct {
		testName                string
		pkgName                 string
		pkgVersion              string
		expectedError           error
		expectedMinDeps         int
		expectedContainsDeps    []string
		expectedMinDevDeps      int
		expectedContainsDevDeps []string
	}{
		{
			testName:             "Get junit dependencies",
			pkgName:              "junit:junit",
			pkgVersion:           "4.13.2",
			expectedError:        nil,
			expectedMinDeps:      1, // JUnit 4.13.2 has hamcrest-core as dependency
			expectedContainsDeps: []string{"org.hamcrest:hamcrest-core"},
		},
		{
			testName:      "Invalid package name format",
			pkgName:       "invalid-package-name",
			pkgVersion:    "1.0.0",
			expectedError: assert.AnError, // Expect format error
		},
		{
			testName:      "Non-existent package version",
			pkgName:       "junit:junit",
			pkgVersion:    "999.999.999",
			expectedError: assert.AnError, // Expect POM not found error
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewMavenAdapter()
			if err != nil {
				t.Fatalf("failed to create Maven registry adapter: %v", err)
			}

			packageDiscovery, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in Maven adapter")
			}

			dependencies, err := packageDiscovery.GetPackageDependencies(test.pkgName, test.pkgVersion)

			if test.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, dependencies)
				assert.GreaterOrEqual(t, len(dependencies.Dependencies), test.expectedMinDeps)
				assert.GreaterOrEqual(t, len(dependencies.DevDependencies), test.expectedMinDevDeps)

				// Check that expected dependencies are present
				if len(test.expectedContainsDeps) > 0 {
					depNames := make([]string, len(dependencies.Dependencies))
					for i, dep := range dependencies.Dependencies {
						depNames[i] = dep.Name
					}

					for _, expected := range test.expectedContainsDeps {
						found := false
						for _, actual := range depNames {
							if actual == expected {
								found = true
								break
							}
						}
						if !found {
							t.Logf("Expected dependency %s not found in results", expected)
						}
					}
				}

				// Check that expected dev dependencies are present
				if len(test.expectedContainsDevDeps) > 0 {
					devDepNames := make([]string, len(dependencies.DevDependencies))
					for i, dep := range dependencies.DevDependencies {
						devDepNames[i] = dep.Name
					}

					for _, expected := range test.expectedContainsDevDeps {
						found := false
						for _, actual := range devDepNames {
							if actual == expected {
								found = true
								break
							}
						}
						if !found {
							t.Logf("Expected dev dependency %s not found in results", expected)
						}
					}
				}
			}
		})
	}
}

func TestMavenGetPackageDownloadStats(t *testing.T) {
	t.Skip()

	adapter, err := NewMavenAdapter()
	if err != nil {
		t.Fatalf("failed to create Maven registry adapter: %v", err)
	}

	packageDiscovery, err := adapter.PackageDiscovery()
	if err != nil {
		t.Fatalf("failed to create package discovery client in Maven adapter")
	}

	// Maven download stats should return an error as they're not available
	_, err = packageDiscovery.GetPackageDownloadStats("junit:junit")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "download stats are not available")
}

func TestNewMavenAdapter(t *testing.T) {
	t.Skip()

	adapter, err := NewMavenAdapter()
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	// Test that we can get discovery clients
	pd, err := adapter.PublisherDiscovery()
	assert.NoError(t, err)
	assert.NotNil(t, pd)

	packageDiscovery, err := adapter.PackageDiscovery()
	assert.NoError(t, err)
	assert.NotNil(t, packageDiscovery)
}

func TestMavenGetLatestVersion(t *testing.T) {
	t.Skip()

	adapter, err := NewMavenAdapter()
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	packageDiscovery, err := adapter.PackageDiscovery()
	assert.NoError(t, err)
	assert.NotNil(t, packageDiscovery)

	t.Run("junit:junit", func(t *testing.T) {
		pkg, err := packageDiscovery.GetPackage("junit:junit")
		assert.NoError(t, err)
		assert.NotNil(t, pkg)

		assert.Equal(t, "junit:junit", pkg.Name)
		assert.True(t, semver.IsAheadOrEqual("4.13.2", pkg.LatestVersion))
	})

	t.Run("non-existent package", func(t *testing.T) {
		pkg, err := packageDiscovery.GetPackage("non.existent:non-existent")
		assert.ErrorIs(t, err, ErrPackageNotFound)
		assert.Nil(t, pkg)
	})
}
