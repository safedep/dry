package obs

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type promMetricReceiver struct {
	counter prometheus.Counter
}

func (r *promMetricReceiver) Inc() {
	r.counter.Inc()
}

func (r *promMetricReceiver) Add(delta float64) {
	r.counter.Add(delta)
}

type promGaugeReceiver struct {
	gauge prometheus.Gauge
}

func (r *promGaugeReceiver) Set(value float64) {
	r.gauge.Set(value)
}

func (r *promGaugeReceiver) Add(delta float64) {
	r.gauge.Add(delta)
}

func (r *promGaugeReceiver) Sub(delta float64) {
	r.gauge.Sub(delta)
}

type prometheusMetricsProvider struct {
	namespace string
	subsystem string
}

type promHistogramReceiver struct {
	histogram prometheus.Histogram
}

func (r *promHistogramReceiver) Observe(value float64) {
	r.histogram.Observe(value)
}

// NewPrometheusMetricsProvider creates a new Provider that uses Prometheus
// Go SDK to create metrics.
func NewPrometheusMetricsProvider(namespace, subsystem string) Provider {
	return &prometheusMetricsProvider{
		namespace: strings.ReplaceAll(namespace, "-", "_"),
		subsystem: strings.ReplaceAll(subsystem, "-", "_"),
	}
}

func (p *prometheusMetricsProvider) NewCounter(name, desc string) Counter {
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: p.namespace,
		Subsystem: p.subsystem,
		Name:      name,
		Help:      desc,
	})

	prometheus.MustRegister(c)
	return &promMetricReceiver{
		counter: c,
	}
}

func (p *prometheusMetricsProvider) NewGauge(name, desc string) Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: p.namespace,
		Subsystem: p.subsystem,
		Name:      name,
		Help:      desc,
	})

	prometheus.MustRegister(g)
	return &promGaugeReceiver{
		gauge: g,
	}
}

func (p *prometheusMetricsProvider) NewHistogram(name, desc string,
	opts ...ProviderSpecificOptsEditor) Histogram {
	histogramOpts := prometheus.HistogramOpts{
		Namespace: p.namespace,
		Subsystem: p.subsystem,
		Name:      name,
		Help:      desc,
	}

	for _, editor := range opts {
		editor(&histogramOpts)
	}

	return &promHistogramReceiver{
		histogram: prometheus.NewHistogram(histogramOpts),
	}
}
