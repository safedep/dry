package obs

import (
	"maps"
	"sync"
	"sync/atomic"
)

// deferredProvider is the provider in place before InitPrometheusMetricsProvider
// runs. Go initialises packages in dependency order and breaks ties by import
// path, so a blank import of the package that calls
// InitPrometheusMetricsProvider does not make it run first: a package-level
// `var m = obs.NewCounter(...)` in a package that initialises earlier ran
// against the no-op provider and stayed a no-op for the life of the process.
// The deferred provider records each declaration and binds it to the real
// metric when InitPrometheusMetricsProvider runs. A declaration that arrives
// after the bind is created on the real provider at once. Calls on a metric
// before its bind are dropped, which matches the old behaviour for that
// window.
type deferredProvider struct {
	mu      sync.Mutex
	real    Provider
	pending []func(Provider)
}

func newDeferredProvider() *deferredProvider {
	return &deferredProvider{}
}

// defer_ records a declaration, or runs it at once when the provider is
// already bound. The check and the append happen under the same lock as
// bind, so a declaration cannot slip between the drain and the switch.
func (p *deferredProvider) defer_(bind func(Provider)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.real != nil {
		bind(p.real)
		return
	}
	p.pending = append(p.pending, bind)
}

// bind materialises every recorded declaration with the real provider and
// routes later declarations to it directly.
func (p *deferredProvider) bind(real Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.real = real
	for _, b := range p.pending {
		b(real)
	}
	p.pending = nil
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
// taken before the bind starts counting after it. The labels are copied,
// because the Prometheus vec selects its child at WithLabels time and the
// caller may reuse the map.
func (v *deferredCounterVec) WithLabels(labels map[string]string) Counter {
	if r := v.real.Load(); r != nil {
		return (*r).WithLabels(labels)
	}
	return &deferredLabelledCounter{vec: v, labels: maps.Clone(labels)}
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
	return &deferredLabelledGauge{vec: v, labels: maps.Clone(labels)}
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
