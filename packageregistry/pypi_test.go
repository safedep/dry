package packageregistry

import (
	"github.com/safedep/dry/semver"
	"reflect"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestPypiGetPublisher(t *testing.T) {
	cases := []struct {
		name       string
		pkgName    string
		pkgVersion string

		expectedError      error
		expectedPublishers []Publisher
	}{
		{
			name:       "pypi package django",
			pkgName:    "django",
			pkgVersion: "5.1.5",
			expectedPublishers: []Publisher{
				{Name: "", Email: "Django Software Foundation <foundation@djangoproject.com>"},
			},
		},
		{
			name:       "pypi package numpy",
			pkgName:    "numpy",
			pkgVersion: "1.2.0",
			expectedPublishers: []Publisher{
				{Name: "NumPy Developers", Email: "numpy-discussion@scipy.org"},
			},
		},
		{
			name:          "Incorrect package version",
			pkgName:       "@adguard/dnr-rulesets",
			pkgVersion:    "0.0.0",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := NewPypiAdapter()
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
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.expectedError.Error())
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

func TestPypiGetPackage(t *testing.T) {
	cases := []struct {
		pkgName string

		expectedPackageName string
		expectedAuthorName  string
		expectedAuthorEmail string
		expectedMaintainers int
		expectedMinVersions int
		expectedRepoURL     string
		expectedError       error
		assert              func(t *testing.T, pkg *Package)
	}{
		{
			pkgName: "requests",

			expectedPackageName: "requests",
			expectedAuthorName:  "Kenneth Reitz",
			expectedAuthorEmail: "me@kennethreitz.org",
			expectedMaintainers: 0,
			expectedMinVersions: 30,
			expectedRepoURL:     "https://github.com/psf/requests",
			expectedError:       nil,
		},
		{
			pkgName: "django",

			expectedPackageName: "Django",
			expectedAuthorName:  "",
			expectedAuthorEmail: "Django Software Foundation <foundation@djangoproject.com>",
			expectedMaintainers: 0,
			expectedMinVersions: 50,
			expectedRepoURL:     "https://github.com/django/django",
			expectedError:       nil,
		},
		{
			pkgName:       "nonexistent",
			expectedError: ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewPypiAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry pypi adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in pypi adapter")
			}

			pkg, err := pd.GetPackage(test.pkgName)
			if test.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)

				assert.Equal(t, test.expectedPackageName, pkg.Name)
				assert.Equal(t, test.expectedAuthorName, pkg.Author.Name)
				assert.Equal(t, test.expectedAuthorEmail, pkg.Author.Email)
				assert.Equal(t, test.expectedMaintainers, len(pkg.Maintainers))
				assert.GreaterOrEqual(t, len(pkg.Versions), test.expectedMinVersions)
				assert.Equal(t, test.expectedRepoURL, pkg.SourceRepositoryUrl)
			}
		})
	}
}

// Getting packages by publisher is not supported in pypi
func TestPypiGetPublisherPackages(t *testing.T) {
	cases := []struct {
		publisher     Publisher
		expectedError error
	}{
		{
			publisher:     Publisher{Name: "Kenneth"},
			expectedError: ErrNoPackagesFound,
		},
	}

	for _, test := range cases {
		t.Run(test.publisher.Name, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewPypiAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry pypi adapter: %v", err)
			}

			pd, err := adapter.PublisherDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in pypi adapter")
			}

			packages, err := pd.GetPublisherPackages(test.publisher)
			if test.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, packages)
			}
		})
	}
}

func TestPypiGetPackageLatestVersion(t *testing.T) {
	cases := []struct {
		pkgName               string
		expectedError         error
		expectedLatestVersion string
	}{
		{
			pkgName:               "requests",
			expectedLatestVersion: "2.32.3",
		},
		{
			pkgName:               "fastapi",
			expectedLatestVersion: "0.115.12",
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewPypiAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry pypi adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in pypi adapter")
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
