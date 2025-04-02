package packageregistry

import (
	"fmt"
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
		publishers []*Publisher
	}{
		{
			name:    "ruby gem gemcutter",
			pkgName: "gemcutter",
			publishers: []*Publisher{
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
			err:        fmt.Errorf("unable to fetch ruby package metadata"),
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

func TestRubyGetPackages(t *testing.T) {
	cases := []struct {
		name           string
		publishername  string
		publisherEmail string
		minPackages    int
		err            error
	}{
		{
			name:           "Correct ruby publisher",
			publishername:  "sferik",
			publisherEmail: "sferik@gmail.com",
			minPackages:    2,
		},
		{
			name:           "incorrect publisher info",
			publishername:  "randomrubypackage",
			publisherEmail: "rndom.com",
			err:            fmt.Errorf("packages not found for author"),
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
