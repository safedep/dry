package packageregistry

import (
	"reflect"
	"testing"

	"github.com/safedep/dry/semver"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestNpmGetPublisher(t *testing.T) {
	cases := []struct {
		testName   string
		pkgName    string
		pkgVersion string

		expectedError      error
		expectedPublishers []Publisher
		assertFunc         func(t *testing.T, publisherInfo *PackagePublisherInfo, err error)
	}{
		{
			testName:   "Correct Npm publisher for package",
			pkgName:    "@kunalsin9h/load-gql",
			pkgVersion: "1.0.2",

			expectedError:      nil,
			expectedPublishers: []Publisher{{Name: "kunalsin9h", Email: "kunalsin9h@gmail.com"}},
		},
		{
			testName:   "Correct NPM publisher for package express",
			pkgName:    "express",
			pkgVersion: "5.1.0",

			expectedError: nil,
			expectedPublishers: []Publisher{
				{Name: "wesleytodd", Email: "wes@wesleytodd.com"},
				{Name: "jonchurch", Email: "npm@jonchurch.com"},
				{Name: "ctcpip", Email: "c@labsector.com"},
				{Name: "sheplu", Email: "jean.burellier@gmail.com"},
			},
		},
		{
			testName:      "Failed to fetch package",
			pkgName:       "@adguard/dnr-rulesets",
			pkgVersion:    "0.0.0",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewNpmAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}
			pd, err := adapter.PublisherDiscovery()

			if err != nil {
				t.Fatalf("failed to create publisher discovery client in npm adapter")
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
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.Equal(t, len(publisherInfo.Publishers), len(test.expectedPublishers))

				if !reflect.DeepEqual(publisherInfo.Publishers, test.expectedPublishers) {
					t.Errorf("expected: %v, got: %v", test.expectedPublishers, publisherInfo.Publishers)
				}
			}
		})
	}
}

func TestNpmGetPackagesByPublisher(t *testing.T) {
	cases := []struct {
		testName       string
		publishername  string
		publisherEmail string

		expectedError       error
		expectedMinPackages int
		expectedPkgNames    []string
	}{
		{
			testName:       "Correct Npm publisher",
			publishername:  "kunalsin9h",
			publisherEmail: "kunal@kunalsin9h.com",

			expectedError:       nil,
			expectedMinPackages: 2,
			expectedPkgNames:    []string{"@kunalsin9h/load-gql", "instant-solid"},
		},
		{
			testName:       "incorrect publisher info",
			publishername:  "randomguyyssssss",
			publisherEmail: "randomguyyssssss@gmail.com",

			expectedError: ErrNoPackagesFound,
		},
	}

	for _, test := range cases {
		t.Run(test.testName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewNpmAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}

			pd, err := adapter.PublisherDiscovery()
			if err != nil {
				t.Fatalf("failed to create publisher discovery client in npm adapter")
			}

			pkgs, err := pd.GetPublisherPackages(Publisher{Name: test.publishername, Email: test.publisherEmail})
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.expectedMinPackages)
				for _, pkg := range pkgs {
					assert.Contains(t, test.expectedPkgNames, pkg.Name)
				}
			}
		})
	}
}

func TestNpmPackageDiscoveryDownloadStats(t *testing.T) {
	cases := []struct {
		pkgName                     string
		expectedError               error
		expectedMinDailyDownloads   uint64
		expectedMinWeeklyDownloads  uint64
		expectedMinMonthlyDownloads uint64
		expectedMinTotalDownloads   uint64
	}{
		{
			pkgName:                     "express",
			expectedError:               nil,
			expectedMinDailyDownloads:   1000_000,
			expectedMinWeeklyDownloads:  7000_000,
			expectedMinMonthlyDownloads: 30_000_000,
			expectedMinTotalDownloads:   1_000_000_000, // express downloads on last year, we will check >= this
		},
		{
			pkgName:                     "@kunalsin9h/load-gql",
			expectedError:               nil,
			expectedMinDailyDownloads:   0,
			expectedMinWeeklyDownloads:  0,
			expectedMinMonthlyDownloads: 0,
			expectedMinTotalDownloads:   50,
		},
		{
			pkgName:       "random-package-name-that-does-not-exist-1246890",
			expectedError: ErrPackageNotFound,
		},
	}
	packageDiscovery := npmPackageDiscovery{}
	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			downloadStats, err := packageDiscovery.GetPackageDownloadStats(test.pkgName)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, downloadStats)

				// Downloads data
				assert.GreaterOrEqual(t, downloadStats.Daily, test.expectedMinDailyDownloads)
				assert.GreaterOrEqual(t, downloadStats.Weekly, test.expectedMinWeeklyDownloads)
				assert.GreaterOrEqual(t, downloadStats.Monthly, test.expectedMinMonthlyDownloads)
				assert.GreaterOrEqual(t, downloadStats.Total, test.expectedMinTotalDownloads)
			}
		})
	}
}

func TestNpmGetPackage(t *testing.T) {
	cases := []struct {
		pkgName string

		expectedError        error
		expectedMinDownloads uint64
		expectedRepoURL      string
		expectedPublishers   Publisher
	}{
		{
			pkgName:              "express",
			expectedError:        nil,
			expectedMinDownloads: 1658725727, // express downoads on last year, we will check >= this
			expectedRepoURL:      "https://github.com/expressjs/express",
			expectedPublishers: Publisher{
				Name:  "TJ Holowaychuk",
				Email: "tj@vision-media.ca",
			},
		},
		{
			pkgName:              "@kunalsin9h/load-gql",
			expectedError:        nil,
			expectedMinDownloads: 50,
			expectedRepoURL:      "https://github.com/kunalsin9h/load-gql",
			expectedPublishers: Publisher{
				Name:  "Kunal Singh",
				Email: "kunal@kunalsin9h.com",
			},
		},

		{
			pkgName:       "random-package-name-that-does-not-exist-1246890",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewNpmAdapter()

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
				// Downloads data
				assert.True(t, pkg.Downloads.Valid)
				assert.GreaterOrEqual(t, pkg.Downloads.Value, test.expectedMinDownloads)

				// Repository data
				assert.Equal(t, test.expectedRepoURL, pkg.SourceRepositoryUrl)

				// Publisher data
				assert.Equal(t, test.expectedPublishers.Name, pkg.Author.Name)
				assert.Equal(t, test.expectedPublishers.Email, pkg.Author.Email)
			}
		})
	}
}

func TestNpmGetPackageLatestVersion(t *testing.T) {
	cases := []struct {
		pkgName               string
		expectedError         error
		expectedLatestVersion string
	}{
		{
			pkgName:               "express",
			expectedError:         nil,
			expectedLatestVersion: "5.1.0",
		},
		{
			pkgName:               "sql",
			expectedLatestVersion: "0.78.0",
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewNpmAdapter()
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
				assert.True(t, semver.IsAheadOrEqual(test.expectedLatestVersion, pkg.LatestVersion))
			}
		})
	}
}

func TestNpmGetPackageDependencies(t *testing.T) {
	cases := []struct {
		name       string
		pkgName    string
		pkgVersion string
		assertFn   func(t *testing.T, dependencies *PackageDependencyList, err error)
	}{
		{
			name:       "valid express package",
			pkgName:    "express",
			pkgVersion: "4.17.1",
			assertFn: func(t *testing.T, dependencies *PackageDependencyList, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 30, len(dependencies.Dependencies))
				assert.Contains(t, dependencies.Dependencies, PackageDependencyInfo{
					Name:        "debug",
					VersionSpec: "2.6.9",
				})
				assert.Contains(t, dependencies.DevDependencies, PackageDependencyInfo{
					Name:        "cookie-parser",
					VersionSpec: "~1.4.4",
				})
			},
		},
		{
			name:       "invalid package",
			pkgName:    "invalid-package-name",
			pkgVersion: "1.0.0",
			assertFn: func(t *testing.T, dependencies *PackageDependencyList, err error) {
				assert.ErrorIs(t, err, ErrPackageNotFound)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewNpmAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in npm adapter")
			}

			dependencies, err := pd.GetPackageDependencies(test.pkgName, test.pkgVersion)
			test.assertFn(t, dependencies, err)
		})
	}
}
