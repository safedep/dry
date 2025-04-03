package packageregistry

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestNpmGetPublisher(t *testing.T) {
	cases := []struct {
		testName   string
		pkgName    string
		pkgVersion string
		err        error
		publishers []*Publisher
		assertFunc func(t *testing.T, publisherInfo *PackagePublisherInfo, err error)
	}{
		{
			testName:   "Correct Npm publisher for package",
			pkgName:    "@adguard/dnr-rulesets",
			pkgVersion: "1.2.20250128090114",
			assertFunc: func(t *testing.T, publisherInfo *PackagePublisherInfo, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.Equal(t, 3, len(publisherInfo.Publishers)) // 3 Maintainers

				// The name and email are public data available from the npm registry
				assert.ElementsMatch(t, []*Publisher{
					{Name: "ameshkov", Email: "am@adguard.com"},
					{Name: "maximtop", Email: "maximtop@gmail.com"},
					{Name: "blakhard", Email: "vlad.abdulmianov@gmail.com"},
				}, publisherInfo.Publishers)
			},
		},
		{
			testName:   "Failed to fetch package",
			pkgName:    "@adguard/dnr-rulesets",
			pkgVersion: "0.0.0",
			err:        ErrFailedToFetchPackage,
			assertFunc: func(t *testing.T, publisherInfo *PackagePublisherInfo, err error) {
				assert.ErrorIs(t, err, ErrFailedToFetchPackage)
				assert.Nil(t, publisherInfo)
			},
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
			test.assertFunc(t, publisherInfo, err)
		})
	}

}

func TestNpmGetPackagesByPublisher(t *testing.T) {
	cases := []struct {
		testName       string
		publishername  string
		publisherEmail string
		minPackages    int
		err            error
		pkgNames       []string
	}{
		{
			testName:       "Correct Npm publisher",
			publishername:  "kunalsin9h",
			publisherEmail: "kunal@kunalsin9h.com",
			minPackages:    2,
			err:            nil,
			pkgNames:       []string{"@kunalsin9h/load-gql", "instant-solid"},
		},
		{
			testName:       "incorrect publisher info",
			publishername:  "randomguyyssssss",
			publisherEmail: "randomguyyssssss@gmail.com",
			err:            ErrNoPackagesFound,
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
			if test.err != nil {
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.minPackages)
				for _, pkg := range pkgs {
					assert.Contains(t, test.pkgNames, pkg.Name)
				}
			}
		})
	}
}

func TestNpmGetPackage(t *testing.T) {
	cases := []struct {
		pkgName    string
		err        error
		downloads  uint64
		repoURL    string
		publishers Publisher
	}{
		{
			pkgName:   "express",
			err:       nil,
			downloads: 1658725727, // express downoads on last year, we will check >= this
			repoURL:   "git+https://github.com/expressjs/express.git",
			publishers: Publisher{
				Name:  "TJ Holowaychuk",
				Email: "tj@vision-media.ca",
			},
		},
		{
			pkgName:   "@kunalsin9h/load-gql",
			err:       nil,
			downloads: 90,
			repoURL:   "git+https://github.com/kunalsin9h/load-gql.git",
			publishers: Publisher{
				Name:  "Kunal Singh",
				Email: "kunal@kunalsin9h.com",
			},
		},

		{
			pkgName: "random-package-name-that-does-not-exist-1246890",
			err:     ErrPackageNotFound,
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
			if test.err != nil {
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)
				// Downloads data
				assert.True(t, pkg.Downloads.Valid)
				assert.GreaterOrEqual(t, pkg.Downloads.Value, test.downloads)

				// Repository data
				assert.Equal(t, test.repoURL, pkg.SourceRepositoryUrl)
			}
		})
	}
}
