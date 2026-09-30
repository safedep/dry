package obs

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusMetricsProvider(t *testing.T) {
	p := NewPrometheusMetricsProvider("test", "test")

	c := p.NewCounter("test_c_1", "test")
	c.Add(1)
	c.Inc()

	g := p.NewGauge("test_g_1", "test")
	g.Add(1)
	g.Set(1)
	g.Sub(1)

	h := p.NewHistogram("test_h_1", "test")
	h.Observe(1)
}

func TestPrometheusCounterVec(t *testing.T) {
	p := NewPrometheusMetricsProvider("test", "test")
	c := p.NewCounterVec("test_vec_a_1", "test", []string{"label1", "label2"})

	c.WithLabels(map[string]string{"label1": "1", "label2": "2"}).Inc()
	c.WithLabels(map[string]string{"label1": "1", "label2": "2"}).Add(1)
}

func TestPrometheusGaugeVec(t *testing.T) {
	p := NewPrometheusMetricsProvider("test", "test")
	g := p.NewGaugeVec("test_gauge_vec_1", "test", []string{"label1", "label2"})

	g.WithLabels(map[string]string{"label1": "1", "label2": "2"}).Set(1)
	g.WithLabels(map[string]string{"label1": "1", "label2": "2"}).Add(1)
	g.WithLabels(map[string]string{"label1": "1", "label2": "2"}).Sub(1)
}

func TestPrometheusHistogramBuckets(t *testing.T) {
	p := NewPrometheusMetricsProvider("test", "test")
	bounds := []float64{10, 100, 1000}

	h := p.NewHistogram("test_h_buckets_1", "test", withHistogramBuckets(bounds))
	h.Observe(50)

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	var got []float64
	for _, family := range families {
		if family.GetName() != "test_test_test_h_buckets_1" {
			continue
		}
		for _, bucket := range family.GetMetric()[0].GetHistogram().GetBucket() {
			got = append(got, bucket.GetUpperBound())
		}
	}

	assert.Equal(t, bounds, got)
}
