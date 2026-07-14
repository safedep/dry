package obs

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gathered reports whether a metric family with the given fully-qualified
// name is present in the default registry.
func gathered(t *testing.T, name string) bool {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return true
		}
	}
	return false
}

// TestPrometheusHistogramIsRegistered guards against a regression where
// NewHistogram constructed a histogram but never registered it, leaving its
// _bucket/_sum/_count series unscraped. Counter/Gauge/Vec are already covered
// implicitly by their MustRegister calls; this pins the same for histograms.
func TestPrometheusHistogramIsRegistered(t *testing.T) {
	p := NewPrometheusMetricsProvider("reg", "hist")

	h := p.NewHistogram("registered_1", "test")
	h.Observe(1)

	assert.True(t, gathered(t, "reg_hist_registered_1"),
		"histogram must be registered with the default registry so it is scraped")
}
