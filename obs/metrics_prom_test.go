package obs

import "testing"

func TestPrometheusMetricsProvider(t *testing.T) {
	p := NewPrometheusMetricsProvider("test", "test")
	c := p.NewCounter("test", "test")
	c.Add(1)
	c.Inc()

	g := p.NewGauge("test", "test")
	g.Add(1)
	g.Set(1)
	g.Sub(1)

	h := p.NewHistogram("test", "test")
	h.Observe(1)
}
