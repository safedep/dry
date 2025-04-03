package packageregistry

import (
	"fmt"
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
		err        error
		publishers []Publisher
	}{
		{
			name:       "pypi package django",
			pkgName:    "django",
			pkgVersion: "5.1.5",
			publishers: []Publisher{
				{Name: "", Email: "Django Software Foundation <foundation@djangoproject.com>"},
			},
		},
		{
			name:       "pypi package numpy",
			pkgName:    "numpy",
			pkgVersion: "1.2.0",
			publishers: []Publisher{
				{Name: "NumPy Developers", Email: "numpy-discussion@scipy.org"},
			},
		},
		{
			name:       "Incorrect package version",
			pkgName:    "@adguard/dnr-rulesets",
			pkgVersion: "0.0.0",
			err:        ErrPackageNotFound,
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

			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.Equal(t, len(publisherInfo.Publishers), len(test.publishers))

				if !reflect.DeepEqual(publisherInfo.Publishers, test.publishers) {
					t.Errorf("expected: %v, got: %v", test.publishers, publisherInfo.Publishers)
				}
			}
		})
	}

}

func TestPypiGetPackage(t *testing.T) {
	cases := []struct {
		pkgName string
		err     error
		assert  func(t *testing.T, pkg *Package)
	}{
		{
			pkgName: "requests",
			assert: func(t *testing.T, pkg *Package) {
				assert.Equal(t, pkg.Name, "requests")
				assert.Equal(t, pkg.Author.Name, "Kenneth Reitz")
				assert.Equal(t, pkg.Author.Email, "me@kennethreitz.org")
				fmt.Printf("pkg.Maintainers: %+v\n", pkg.Maintainers)
				assert.Equal(t, len(pkg.Maintainers), 0) // No maintainer for requests package
				assert.Equal(t, pkg.SourceRepositoryUrl, "https://github.com/psf/requests")
				assert.GreaterOrEqual(t, len(pkg.Versions), 30) // requests has more than 30 versions
			},
		},
		{
			pkgName: "django",
			assert: func(t *testing.T, pkg *Package) {
				assert.Equal(t, pkg.Name, "Django")
				assert.Equal(t, pkg.Author.Name, "") // No author for django package
				assert.Equal(t, pkg.Author.Email, "Django Software Foundation <foundation@djangoproject.com>")
				assert.Equal(t, len(pkg.Maintainers), 0) // No maintainer for django package
				assert.Equal(t, pkg.SourceRepositoryUrl, "https://github.com/django/django")
				assert.GreaterOrEqual(t, len(pkg.Versions), 50) // django has more than 50 versions
			},
		},
		{
			pkgName: "nonexistent",
			err:     ErrPackageNotFound,
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
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)
				test.assert(t, pkg)
			}
		})
	}
}

func TestPypiGetPublisherPackages(t *testing.T) {
	cases := []struct {
		publisher Publisher
		err       error
	}{
		{
			publisher: Publisher{Name: "Kenneth"},
			err:       ErrNoPackagesFound,
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
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, packages)
			}
		})
	}
}
