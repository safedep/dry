package endpointsync

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReportedLabels(t *testing.T) {
	cases := []struct {
		goos, goarch string
	}{
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"windows", "amd64"},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			labels := reportedLabels(tc.goos, tc.goarch)
			got := map[string]string{}
			for _, l := range labels {
				got[l.GetKey()] = l.GetValue()
			}
			assert.Equal(t, map[string]string{"reported/os": tc.goos, "reported/arch": tc.goarch}, got)
		})
	}
}

func TestReportedLabels_UsesTheRuntime(t *testing.T) {
	labels := ReportedLabels()
	assert.Len(t, labels, 2)
	assert.Equal(t, LabelReportedOS, labels[0].GetKey())
	assert.Equal(t, runtime.GOOS, labels[0].GetValue())
	assert.Equal(t, LabelReportedArch, labels[1].GetKey())
	assert.Equal(t, runtime.GOARCH, labels[1].GetValue())
}
