package packageregistry

import (
	"reflect"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestRubyGetPublisher(t *testing.T) {
	cases := []struct {
		name       string
		pkgName    string
		pkgVersion string
		err        error
		publishers []Publisher
	}{
		{
			name:    "ruby gem gemcutter",
			pkgName: "gemcutter",
			publishers: []Publisher{
				{Name: "qrush", Email: ""},
				{Name: "sferik", Email: "sferik@gmail.com"},
				{Name: "gemcutter", Email: ""},
				{Name: "techpickles", Email: ""},
			},
		},
		{
			name:       "Incorrect package name",
			pkgName:    "railsii",
			pkgVersion: "0.0.0",
			err:        ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := NewRubyAdapter()
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

func TestRubyGetPublisherPackages(t *testing.T) {
	cases := []struct {
		name          string
		publishername string
		minPackages   int
		err           error
	}{
		{
			name:          "Correct ruby publisher",
			publishername: "noelrap",
			minPackages:   2,
		},
		{
			name:          "incorrect publisher info",
			publishername: "randomrubypackage",
			err:           ErrAuthorNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewRubyAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}

			pd, err := adapter.PublisherDiscovery()
			if err != nil {
				t.Fatalf("failed to create publisher discovery client in npm adapter")
			}

			pkgs, err := pd.GetPublisherPackages(Publisher{Name: test.publishername, Email: ""})
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.minPackages)
			}
		})
	}

}

func TestRubyGetPackage(t *testing.T) {
	cases := []struct {
		name    string
		pkgName string
		err     error
		assert  func(t *testing.T, pkg *Package)
	}{
		{
			name:    "Correct ruby package",
			pkgName: "rails",
			err:     nil,
			assert: func(t *testing.T, pkg *Package) {
				assert.Equal(t, pkg.Name, "rails")
				assert.GreaterOrEqual(t, len(pkg.Description), 1) // Description is not empty
				assert.Equal(t, pkg.SourceRepositoryUrl, "https://github.com/rails/rails")
				assert.GreaterOrEqual(t, pkg.Downloads.Value, uint64(600000000))
				assert.Equal(t, pkg.Author.Name, "David Heinemeier Hansson")
				assert.GreaterOrEqual(t, len(pkg.Versions), 100) // There are more than 100 versions
			},
		},
		{
			name:    "Incorrect package name",
			pkgName: "railsii",
			err:     ErrPackageNotFound,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			adapter, err := NewRubyAdapter()
			if err != nil {
				t.Fatalf("failed to create package registry npm adapter: %v", err)
			}

			pd, err := adapter.PackageDiscovery()
			if err != nil {
				t.Fatalf("failed to create package discovery client in npm adapter")
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
