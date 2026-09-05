package obs

import (
	"sync"
	"sync/atomic"
)

// deferredProvider is the provider in place before InitPrometheusMetricsProvider
// runs. Go initialises packages in import-path order, so a package-level
// `var m = obs.NewCounter(...)` in a package that sorts before the package that
// calls InitPrometheusMetricsProvider used to run against the no-op provider
// and stay a no-op for the life of the process. malysis adapters/registry/v2
// and control-tower etl/* lost every metric that way. The deferred provider
// records each declaration and binds it to the real metric when
// InitPrometheusMetricsProvider runs. Calls made before the bind are dropped,
// which matches the old behaviour for that window.
type deferredProvider struct {
	mu      sync.Mutex
	pending []func(Provider)
}

func newDeferredProvider() *deferredProvider {
	return &deferredProvider{}
}

func (p *deferredProvider) defer_(bind func(Provider)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = append(p.pending, bind)
}

// bind materialises every recorded declaration with the real provider.
func (p *deferredProvider) bind(real Provider) {
	p.mu.Lock()
	pending := p.pending
	p.pending = nil
	p.mu.Unlock()

	for _, b := range pending {
		b(real)
	}
}

type deferredCounter struct {
	real atomic.Pointer[Counter]
}

func (c *deferredCounter) Inc() {
	if r := c.real.Load(); r != nil {
		(*r).Inc()
	}
}

func (c *deferredCounter) Add(delta float64) {
	if r := c.real.Load(); r != nil {
		(*r).Add(delta)
	}
}

type deferredGauge struct {
	real atomic.Pointer[Gauge]
}

func (g *deferredGauge) Set(value float64) {
	if r := g.real.Load(); r != nil {
		(*r).Set(value)
	}
}

func (g *deferredGauge) Add(delta float64) {
	if r := g.real.Load(); r != nil {
		(*r).Add(delta)
	}
}

func (g *deferredGauge) Sub(delta float64) {
	if r := g.real.Load(); r != nil {
		(*r).Sub(delta)
	}
}

type deferredHistogram struct {
	real atomic.Pointer[Histogram]
}

func (h *deferredHistogram) Observe(value float64) {
	if r := h.real.Load(); r != nil {
		(*r).Observe(value)
	}
}

type deferredCounterVec struct {
	real atomic.Pointer[CounterVec]
}

// WithLabels resolves through the vec on every call, so a labelled counter
// taken before the bind starts counting after it.
func (v *deferredCounterVec) WithLabels(labels map[string]string) Counter {
	if r := v.real.Load(); r != nil {
		return (*r).WithLabels(labels)
	}
	return &deferredLabelledCounter{vec: v, labels: labels}
}

type deferredLabelledCounter struct {
	vec    *deferredCounterVec
	labels map[string]string
}

func (c *deferredLabelledCounter) Inc() {
	if r := c.vec.real.Load(); r != nil {
		(*r).WithLabels(c.labels).Inc()
	}
}

func (c *deferredLabelledCounter) Add(delta float64) {
	if r := c.vec.real.Load(); r != nil {
		(*r).WithLabels(c.labels).Add(delta)
	}
}

type deferredGaugeVec struct {
	real atomic.Pointer[GaugeVec]
}

func (v *deferredGaugeVec) WithLabels(labels map[string]string) Gauge {
	if r := v.real.Load(); r != nil {
		return (*r).WithLabels(labels)
	}
	return &deferredLabelledGauge{vec: v, labels: labels}
}

type deferredLabelledGauge struct {
	vec    *deferredGaugeVec
	labels map[string]string
}

func (g *deferredLabelledGauge) Set(value float64) {
	if r := g.vec.real.Load(); r != nil {
		(*r).WithLabels(g.labels).Set(value)
	}
}

func (g *deferredLabelledGauge) Add(delta float64) {
	if r := g.vec.real.Load(); r != nil {
		(*r).WithLabels(g.labels).Add(delta)
	}
}

func (g *deferredLabelledGauge) Sub(delta float64) {
	if r := g.vec.real.Load(); r != nil {
		(*r).WithLabels(g.labels).Sub(delta)
	}
}

func (p *deferredProvider) NewCounter(name, desc string) Counter {
	c := &deferredCounter{}
	p.defer_(func(real Provider) {
		m := real.NewCounter(name, desc)
		c.real.Store(&m)
	})
	return c
}

func (p *deferredProvider) NewGauge(name, desc string) Gauge {
	g := &deferredGauge{}
	p.defer_(func(real Provider) {
		m := real.NewGauge(name, desc)
		g.real.Store(&m)
	})
	return g
}

func (p *deferredProvider) NewHistogram(name, desc string, opts ...ProviderSpecificOptsEditor) Histogram {
	h := &deferredHistogram{}
	p.defer_(func(real Provider) {
		m := real.NewHistogram(name, desc, opts...)
		h.real.Store(&m)
	})
	return h
}

func (p *deferredProvider) NewCounterVec(name, desc string, labels []string) CounterVec {
	v := &deferredCounterVec{}
	p.defer_(func(real Provider) {
		m := real.NewCounterVec(name, desc, labels)
		v.real.Store(&m)
	})
	return v
}

func (p *deferredProvider) NewGaugeVec(name, desc string, labels []string) GaugeVec {
	v := &deferredGaugeVec{}
	p.defer_(func(real Provider) {
		m := real.NewGaugeVec(name, desc, labels)
		v.real.Store(&m)
	})
	return v
}
