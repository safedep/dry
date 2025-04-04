package packageregistry

import (
	"reflect"
	"testing"

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
			expectedMinDownloads: 90,
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
