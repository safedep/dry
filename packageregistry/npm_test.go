package packageregistry

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/safedep/dry/semver"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		expectedError                 error
		expectedMinDownloads          uint64
		expectedRepoURL               string
		expectedPublishers            Publisher
		expectedVersionForPublishTime string
		expectedPublishTime           string
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
			expectedVersionForPublishTime: "5.1.0",
			expectedPublishTime:           "2025-03-31T14:01:22.509Z",
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
			expectedVersionForPublishTime: "1.0.2",
			expectedPublishTime:           "2023-10-10T17:26:43.220Z",
		},

		{
			pkgName:       "random-package-name-that-does-not-exist-1246890",
			expectedError: ErrPackageNotFound,
		},
		{
			pkgName:       "launch-darkly-js",
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

				// Publish Time
				var version PackageVersionInfo
				for _, v := range pkg.Versions {
					if v.Version == test.expectedVersionForPublishTime {
						version = v
						break
					}
				}

				assert.NotEmpty(t, version)
				expected, err := time.Parse(time.RFC3339Nano, test.expectedPublishTime)
				require.NoError(t, err)

				assert.True(t, version.PublishedAt.Equal(expected), "expected %s, got %s", expected, version.PublishedAt)
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

func TestNpmPackageAuthorUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name           string
		input          string
		expectedAuthor npmPackageAuthor
		expectedErr    error
	}{
		{
			name:  "author as string URL",
			input: `"https://example.com"`,
			expectedAuthor: npmPackageAuthor{
				Url: "https://example.com",
			},
		},
		{
			name:  "author as object with name and email",
			input: `{"name": "John Doe", "email": "john@example.com"}`,
			expectedAuthor: npmPackageAuthor{
				Name:  "John Doe",
				Email: "john@example.com",
			},
		},
		{
			name:  "author as empty string",
			input: `""`,
			expectedAuthor: npmPackageAuthor{
				Url: "",
			},
		},
		{
			name:           "author as empty object",
			input:          `{}`,
			expectedAuthor: npmPackageAuthor{},
		},
		{
			name:  "author as object with only name",
			input: `{"name": "Jane"}`,
			expectedAuthor: npmPackageAuthor{
				Name: "Jane",
			},
		},
		{
			name:        "author as array",
			input:       `["someone"]`,
			expectedErr: ErrFailedToParsePackage,
		},
		{
			name:        "author as number",
			input:       `42`,
			expectedErr: ErrFailedToParsePackage,
		},
		{
			name:        "author as boolean",
			input:       `true`,
			expectedErr: ErrFailedToParsePackage,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var author npmPackageAuthor
			err := json.Unmarshal([]byte(tc.input), &author)
			if tc.expectedErr != nil {
				assert.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedAuthor, author)
			}
		})
	}
}

func TestNpmPackageTimeUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name             string
		input            string
		expectedCreated  string
		expectedModified string
		expectedVersions map[string]string
		expectedErr      bool
	}{
		{
			name: "normal time with created, modified, and versions",
			input: `{
				"created": "2023-01-01T00:00:00.000Z",
				"modified": "2023-06-15T12:30:00.000Z",
				"1.0.0": "2023-01-01T00:00:00.000Z",
				"1.1.0": "2023-03-15T10:00:00.000Z"
			}`,
			expectedCreated:  "2023-01-01T00:00:00Z",
			expectedModified: "2023-06-15T12:30:00Z",
			expectedVersions: map[string]string{
				"1.0.0": "2023-01-01T00:00:00Z",
				"1.1.0": "2023-03-15T10:00:00Z",
			},
		},
		{
			name: "time with unpublished object is skipped gracefully",
			input: `{
				"created": "2023-01-01T00:00:00.000Z",
				"modified": "2023-06-15T12:30:00.000Z",
				"unpublished": {
					"name": "some-user",
					"versions": ["1.0.0"],
					"time": "2023-06-15T12:30:00.000Z"
				}
			}`,
			expectedCreated:  "2023-01-01T00:00:00Z",
			expectedModified: "2023-06-15T12:30:00Z",
			expectedVersions: map[string]string{},
		},
		{
			name:             "empty time object",
			input:            `{}`,
			expectedVersions: map[string]string{},
		},
		{
			name: "time with RFC3339 (no fractional seconds)",
			input: `{
				"created": "2023-01-01T00:00:00Z",
				"1.0.0": "2023-01-01T00:00:00Z"
			}`,
			expectedCreated:  "2023-01-01T00:00:00Z",
			expectedVersions: map[string]string{"1.0.0": "2023-01-01T00:00:00Z"},
		},
		{
			name:        "time with invalid date string",
			input:       `{"created": "not-a-date"}`,
			expectedErr: true,
		},
		{
			name:        "time as non-object",
			input:       `"just a string"`,
			expectedErr: true,
		},
		{
			name:             "time as null results in zero values",
			input:            `null`,
			expectedVersions: map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var pt npmPackageTime
			err := json.Unmarshal([]byte(tc.input), &pt)
			if tc.expectedErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.expectedCreated != "" {
				expected, err := time.Parse(time.RFC3339, tc.expectedCreated)
				require.NoError(t, err)
				assert.True(t, pt.Created.Equal(expected), "created: expected %s, got %s", expected, pt.Created)
			}
			if tc.expectedModified != "" {
				expected, err := time.Parse(time.RFC3339, tc.expectedModified)
				require.NoError(t, err)
				assert.True(t, pt.Modified.Equal(expected), "modified: expected %s, got %s", expected, pt.Modified)
			}
			assert.Equal(t, len(tc.expectedVersions), len(pt.Versions))
			for ver, ts := range tc.expectedVersions {
				expected, err := time.Parse(time.RFC3339, ts)
				require.NoError(t, err)
				actual, ok := pt.Versions[ver]
				assert.True(t, ok, "version %s not found", ver)
				assert.True(t, actual.Equal(expected), "version %s: expected %s, got %s", ver, expected, actual)
			}
		})
	}
}

func TestNpmPackageUnmarshalJSON(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		assertFn    func(t *testing.T, pkg *npmPackage)
		expectedErr bool
	}{
		{
			name: "minimal valid package",
			input: `{
				"name": "test-pkg",
				"description": "A test package",
				"versions": {
					"1.0.0": {"version": "1.0.0"}
				},
				"dist-tags": {"latest": "1.0.0"},
				"author": {"name": "Test Author", "email": "test@example.com"},
				"repository": {"url": "git+https://github.com/test/test.git", "type": "git"},
				"maintainers": [{"name": "maintainer1", "email": "m1@example.com"}],
				"time": {
					"created": "2023-01-01T00:00:00.000Z",
					"modified": "2023-06-01T00:00:00.000Z",
					"1.0.0": "2023-01-01T00:00:00.000Z"
				}
			}`,
			assertFn: func(t *testing.T, pkg *npmPackage) {
				assert.Equal(t, "test-pkg", pkg.Name)
				assert.Equal(t, "A test package", pkg.Description)
				assert.Len(t, pkg.Versions, 1)
				assert.Equal(t, "1.0.0", pkg.Versions["1.0.0"].Version)
				assert.Equal(t, "1.0.0", pkg.DistTags.Latest)
				assert.Equal(t, "Test Author", pkg.Author.Name)
				assert.Equal(t, "test@example.com", pkg.Author.Email)
				assert.Len(t, pkg.Maintainers, 1)
				assert.Equal(t, "maintainer1", pkg.Maintainers[0].Name)
				_, ok := pkg.Time.Versions["1.0.0"]
				assert.True(t, ok)
			},
		},
		{
			name: "package with author as string",
			input: `{
				"name": "test-pkg",
				"versions": {"1.0.0": {"version": "1.0.0"}},
				"dist-tags": {"latest": "1.0.0"},
				"author": "https://example.com/author",
				"repository": {"url": "", "type": "git"},
				"maintainers": [],
				"time": {"created": "2023-01-01T00:00:00.000Z", "modified": "2023-01-01T00:00:00.000Z"}
			}`,
			assertFn: func(t *testing.T, pkg *npmPackage) {
				assert.Equal(t, "https://example.com/author", pkg.Author.Url)
				assert.Empty(t, pkg.Author.Name)
			},
		},
		{
			name: "unpublished package with no versions and unpublished time key",
			input: `{
				"name": "removed-pkg",
				"time": {
					"created": "2023-01-01T00:00:00.000Z",
					"modified": "2023-06-01T00:00:00.000Z",
					"1.0.0": "2023-01-01T00:00:00.000Z",
					"unpublished": {
						"name": "some-user",
						"versions": ["1.0.0"],
						"time": "2023-06-01T00:00:00.000Z",
						"tags": {"latest": "1.0.0"}
					}
				}
			}`,
			assertFn: func(t *testing.T, pkg *npmPackage) {
				assert.Equal(t, "removed-pkg", pkg.Name)
				assert.Empty(t, pkg.Versions)
				assert.False(t, pkg.Time.Created.IsZero())
			},
		},
		{
			name: "package with multiple versions",
			input: `{
				"name": "multi-ver",
				"versions": {
					"1.0.0": {"version": "1.0.0"},
					"1.1.0": {"version": "1.1.0"},
					"2.0.0": {"version": "2.0.0"}
				},
				"dist-tags": {"latest": "2.0.0"},
				"author": {"name": "Author"},
				"repository": {"url": "", "type": "git"},
				"maintainers": [],
				"time": {
					"created": "2023-01-01T00:00:00.000Z",
					"modified": "2023-06-01T00:00:00.000Z",
					"1.0.0": "2023-01-01T00:00:00.000Z",
					"1.1.0": "2023-03-01T00:00:00.000Z",
					"2.0.0": "2023-06-01T00:00:00.000Z"
				}
			}`,
			assertFn: func(t *testing.T, pkg *npmPackage) {
				assert.Len(t, pkg.Versions, 3)
				assert.Equal(t, "2.0.0", pkg.DistTags.Latest)
				assert.Len(t, pkg.Time.Versions, 3)
			},
		},
		{
			name: "package with multiple maintainers having mixed author types",
			input: `{
				"name": "mixed-maintainers",
				"versions": {"1.0.0": {"version": "1.0.0"}},
				"dist-tags": {"latest": "1.0.0"},
				"author": {"name": "Main Author"},
				"repository": {"url": "", "type": "git"},
				"maintainers": [
					{"name": "user1", "email": "user1@example.com"},
					"https://example.com/user2"
				],
				"time": {"created": "2023-01-01T00:00:00.000Z", "modified": "2023-01-01T00:00:00.000Z"}
			}`,
			assertFn: func(t *testing.T, pkg *npmPackage) {
				assert.Len(t, pkg.Maintainers, 2)
				assert.Equal(t, "user1", pkg.Maintainers[0].Name)
				assert.Equal(t, "https://example.com/user2", pkg.Maintainers[1].Url)
			},
		},
		{
			name:        "invalid JSON",
			input:       `not json at all`,
			expectedErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var pkg npmPackage
			err := json.Unmarshal([]byte(tc.input), &pkg)
			if tc.expectedErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tc.assertFn(t, &pkg)
		})
	}
}
