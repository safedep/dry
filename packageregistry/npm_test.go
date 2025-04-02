package packageregistry

import (
	"fmt"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestNpmGetPublisher(t *testing.T) {
	cases := []struct {
		name       string
		pkgName    string
		pkgVersion string
		err        error
		publishers []*Publisher
		assertFunc func(t *testing.T, publisherInfo *PackagePublisherInfo, err error)
	}{
		{
			name:       "npm package dnr-rulesets",
			pkgName:    "@adguard/dnr-rulesets",
			pkgVersion: "1.2.20250128090114",
			assertFunc: func(t *testing.T, publisherInfo *PackagePublisherInfo, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, publisherInfo)
				assert.Equal(t, 3, len(publisherInfo.Publishers))

				// The name and email are public data available from the npm registry
				assert.ElementsMatch(t, []*Publisher{
					{Name: "ameshkov", Email: "am@adguard.com"},
					{Name: "maximtop", Email: "maximtop@gmail.com"},
					{Name: "blakhard", Email: "vlad.abdulmianov@gmail.com"},
				}, publisherInfo.Publishers)
			},
		},
		{
			name:       "Incorrect package version",
			pkgName:    "@adguard/dnr-rulesets",
			pkgVersion: "0.0.0",
			err:        fmt.Errorf("unable to fetch npm package metadata"),
			assertFunc: func(t *testing.T, publisherInfo *PackagePublisherInfo, err error) {
				assert.Error(t, err)
				assert.ErrorContains(t, err, "unable to fetch npm package metadata")
				assert.Nil(t, publisherInfo)
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
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

func TestNpmGetPackages(t *testing.T) {
	cases := []struct {
		name           string
		publishername  string
		publisherEmail string
		minPackages    int
		err            error
	}{
		{
			name:           "Correct Npm publisher",
			publishername:  "maximtop",
			publisherEmail: "maximtop@gmail.com",
			minPackages:    20,
		},
		{
			name:           "incorrect publisher info",
			publishername:  "randomguyyssssss",
			publisherEmail: "rndom.com",
			err:            fmt.Errorf("Packages not found for author"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
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
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkgs)
				assert.GreaterOrEqual(t, len(pkgs), test.minPackages)
			}
		})
	}
}
