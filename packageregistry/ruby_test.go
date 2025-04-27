package packageregistry

import (
	"github.com/safedep/dry/utils"
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

		expectedError      error
		expectedPublishers []Publisher
	}{
		{
			name:    "ruby gem gemcutter",
			pkgName: "gemcutter",

			expectedPublishers: []Publisher{
				{Name: "qrush", Email: ""},
				{Name: "sferik", Email: "sferik@gmail.com"},
				{Name: "gemcutter", Email: ""},
				{Name: "techpickles", Email: ""},
			},
		},
		{
			name:          "Incorrect package name",
			pkgName:       "railsii",
			pkgVersion:    "0.0.0",
			expectedError: ErrPackageNotFound,
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

func TestRubyGetPublisherPackages(t *testing.T) {
	cases := []struct {
		name          string
		publisherName string

		expectedMinPackages int
		expectedError       error
	}{
		{
			name:          "Correct ruby publisher",
			publisherName: "noelrap",

			expectedMinPackages: 2,
		},
		{
			name:          "incorrect publisher info",
			publisherName: "randomrubypackage",
			expectedError: ErrAuthorNotFound,
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

			pkgs, err := pd.GetPublisherPackages(Publisher{Name: test.publisherName, Email: ""})
			if test.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.expectedMinPackages)
			}
		})
	}

}

func TestRubyGetPackage(t *testing.T) {
	cases := []struct {
		name    string
		pkgName string

		expectedPackageName  string
		expectedDescription  bool // Description might change, so we check if is lenght >= 1
		expectedRepoURL      string
		expectedMinDownloads uint64
		expectedAuthorName   string
		expectedMinVersions  int
		expectedError        error
		assert               func(t *testing.T, pkg *Package)
	}{
		{
			name:    "Correct ruby package",
			pkgName: "rails",

			expectedError:        nil,
			expectedPackageName:  "rails",
			expectedDescription:  true,
			expectedRepoURL:      "https://github.com/rails/rails",
			expectedMinDownloads: 600000000,
			expectedAuthorName:   "David Heinemeier Hansson",
			expectedMinVersions:  100,
		},
		{
			name:          "Incorrect package name",
			pkgName:       "railsii",
			expectedError: ErrPackageNotFound,
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
			if test.expectedError != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)

				assert.Equal(t, pkg.Name, test.expectedPackageName)
				if test.expectedDescription {
					assert.GreaterOrEqual(t, len(pkg.Description), 1) // Description is not empty
				}
				assert.Equal(t, pkg.SourceRepositoryUrl, test.expectedRepoURL)
				assert.GreaterOrEqual(t, pkg.Downloads.Value, test.expectedMinDownloads)
				assert.Equal(t, pkg.Author.Name, test.expectedAuthorName)
				assert.GreaterOrEqual(t, len(pkg.Versions), test.expectedMinVersions)
			}
		})
	}
}

func TestRubyGetPackageLatestVersion(t *testing.T) {
	cases := []struct {
		pkgName               string
		expectedError         error
		expectedLatestVersion string
	}{
		{
			pkgName:               "rails",
			expectedLatestVersion: "8.0.2",
		},
		{
			pkgName:               "sql_enum",
			expectedLatestVersion: "1.0.0",
		},
	}

	for _, test := range cases {
		t.Run(test.pkgName, func(t *testing.T) {
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
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.True(t, utils.Version(pkg.LatestVersion).IsGreaterThenOrEqualTo(test.expectedLatestVersion))
			}
		})
	}
}
