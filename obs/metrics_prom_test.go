package obs

import "testing"

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
