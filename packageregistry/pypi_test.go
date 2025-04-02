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
		publishers []*Publisher
	}{
		{
			name:       "pypi package django",
			pkgName:    "django",
			pkgVersion: "5.1.5",
			publishers: []*Publisher{
				{Name: "", Email: "Django Software Foundation <foundation@djangoproject.com>"},
			},
		},
		{
			name:       "Incorrect package version",
			pkgName:    "@adguard/dnr-rulesets",
			pkgVersion: "0.0.0",
			err:        fmt.Errorf("unable to fetch pypi package metadata"),
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
