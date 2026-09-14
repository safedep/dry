package obs

import (
	"fmt"
	"sync"
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

func TestDeferredProviderDeclarationAfterBindIsLive(t *testing.T) {
	d := newDeferredProvider()
	d.bind(NewPrometheusMetricsProvider("deferred_after", ""))

	c := d.NewCounter("after_bind_counter", "declared after bind through the deferred provider")
	c.Inc()

	assert.Equal(t, 1.0, gatherValue(t, "deferred_after_after_bind_counter"),
		"a declaration that races with the bind must reach the real provider")
}

func TestDeferredProviderConcurrentDeclarationsSurviveBind(t *testing.T) {
	d := newDeferredProvider()
	const n = 64
	counters := make([]Counter, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			counters[i] = d.NewCounter(fmt.Sprintf("racing_counter_%d", i), "declared while bind runs")
		}(i)
	}
	close(start)
	d.bind(NewPrometheusMetricsProvider("deferred_race", ""))
	wg.Wait()

	for i, c := range counters {
		c.Inc()
		assert.Equal(t, 1.0, gatherValue(t, fmt.Sprintf("deferred_race_racing_counter_%d", i)), "counter %d was lost", i)
	}
}

func TestDeferredLabelsAreSnapshotted(t *testing.T) {
	d := newDeferredProvider()
	vec := d.NewCounterVec("snapshot_counter_vec", "labels copied at WithLabels", []string{"source"})

	labels := map[string]string{"source": "crates"}
	handle := vec.WithLabels(labels)
	labels["source"] = "npm"

	d.bind(NewPrometheusMetricsProvider("deferred_snapshot", ""))
	handle.Inc()

	mfs, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != "deferred_snapshot_snapshot_counter_vec" {
			continue
		}
		require.Len(t, mf.GetMetric(), 1)
		for _, l := range mf.GetMetric()[0].GetLabel() {
			if l.GetName() == "source" {
				assert.Equal(t, "crates", l.GetValue())
				return
			}
		}
		t.Fatal("source label missing")
	}
	t.Fatal("metric family not gathered")
}
