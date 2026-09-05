package obs

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatherValue returns the sum of every sample in the metric family, or -1
// when the family is absent from the default registry.
func gatherValue(t *testing.T, name string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		total := 0.0
		for _, m := range mf.GetMetric() {
			switch {
			case m.GetCounter() != nil:
				total += m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				total += m.GetGauge().GetValue()
			case m.GetHistogram() != nil:
				total += float64(m.GetHistogram().GetSampleCount())
			}
		}
		return total
	}
	return -1
}

func TestDefaultProviderIsDeferred(t *testing.T) {
	_, ok := __provider.(*deferredProvider)
	assert.True(t, ok, "metrics declared before InitPrometheusMetricsProvider must not be dropped")
}

func TestDeferredProviderBindsEarlyDeclarations(t *testing.T) {
	d := newDeferredProvider()

	counter := d.NewCounter("early_counter", "declared before init")
	gauge := d.NewGauge("early_gauge", "declared before init")
	histogram := d.NewHistogram("early_histogram", "declared before init")
	counterVec := d.NewCounterVec("early_counter_vec", "declared before init", []string{"source"})
	gaugeVec := d.NewGaugeVec("early_gauge_vec", "declared before init", []string{"source"})
	labelledBeforeBind := counterVec.WithLabels(map[string]string{"source": "crates"})

	counter.Inc()
	gauge.Set(5)
	histogram.Observe(1)
	labelledBeforeBind.Inc()
	gaugeVec.WithLabels(map[string]string{"source": "crates"}).Set(3)
	assert.Equal(t, -1.0, gatherValue(t, "deferred_early_counter"), "nothing is registered before the bind")

	d.bind(NewPrometheusMetricsProvider("deferred", ""))

	counter.Inc()
	counter.Add(2)
	gauge.Set(7)
	histogram.Observe(2)
	labelledBeforeBind.Inc()
	counterVec.WithLabels(map[string]string{"source": "npm"}).Add(4)
	gaugeVec.WithLabels(map[string]string{"source": "crates"}).Set(9)
	gaugeVec.WithLabels(map[string]string{"source": "crates"}).Sub(1)

	assert.Equal(t, 3.0, gatherValue(t, "deferred_early_counter"))
	assert.Equal(t, 7.0, gatherValue(t, "deferred_early_gauge"))
	assert.Equal(t, 1.0, gatherValue(t, "deferred_early_histogram"))
	assert.Equal(t, 5.0, gatherValue(t, "deferred_early_counter_vec"))
	assert.Equal(t, 8.0, gatherValue(t, "deferred_early_gauge_vec"))
}

func TestDeferredProviderDeclarationsAfterBindAreDirect(t *testing.T) {
	d := newDeferredProvider()
	d.bind(NewPrometheusMetricsProvider("deferred_late", ""))

	c := d.NewCounter("late_counter", "declared after bind")
	c.Inc()

	assert.Equal(t, -1.0, gatherValue(t, "deferred_late_late_counter"),
		"a deferred provider records declarations only; the caller switches to the real provider after bind")
}
