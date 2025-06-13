package obs

import (
	"fmt"
	dryhttp "github.com/safedep/dry/adapters/http"
)

// Counter is a metric that represents a single numerical value
// that only ever goes up.
type Counter interface {
	Inc()
	Add(float64)
}

// CounterVec is a metric that represents a values with labels.
type CounterVec interface {
	WithLabels(map[string]string) Counter
}

// Gauge is a metric that represents a single numerical value that can
// arbitrarily go up and down.
type Gauge interface {
	Set(float64)
	Add(float64)
	Sub(float64)
}

// Histogram is a metric that samples observations (usually things like
// request durations or response sizes) and counts them in configurable
// buckets. It also provides a sum of all observed values.
type Histogram interface {
	Observe(float64)
}

type ProviderSpecificOpts interface{}
type ProviderSpecificOptsEditor func(ProviderSpecificOpts)

// Provider is an interface that provides a way to create metrics.
// An implementation using Prometheus Go SDK is an example of a Provider.
type Provider interface {
	NewCounter(name, desc string) Counter
	NewGauge(name, desc string) Gauge
	NewHistogram(name, desc string, opts ...ProviderSpecificOptsEditor) Histogram
	NewCounterVec(name, desc string, labels []string) CounterVec
}

type dummyReceiver struct{}

func (d *dummyReceiver) Inc()                                 {}
func (d *dummyReceiver) Set(float64)                          {}
func (d *dummyReceiver) Add(float64)                          {}
func (d *dummyReceiver) Sub(float64)                          {}
func (d *dummyReceiver) Observe(float64)                      {}
func (d *dummyReceiver) WithLabels(map[string]string) Counter { return d }

type dummyProvider struct{}

// NewCounter creates a new Counter.
func (d *dummyProvider) NewCounter(_, _ string) Counter {
	return &dummyReceiver{}
}

// NewGauge creates a new Gauge.
func (d *dummyProvider) NewGauge(_, _ string) Gauge {
	return &dummyReceiver{}
}

// NewHistogram creates a new Histogram.
func (d *dummyProvider) NewHistogram(_, _ string, _ ...ProviderSpecificOptsEditor) Histogram {
	return &dummyReceiver{}
}

// NewCounterVec creates a new CounterVec.
func (d *dummyProvider) NewCounterVec(_, _ string, _ []string) CounterVec {
	return &dummyReceiver{}
}

var (
	__provider Provider = &dummyProvider{}
)

func NewCounter(name, desc string) Counter {
	return __provider.NewCounter(name, desc)
}

func NewCounterVec(name, desc string, labels []string) CounterVec {
	return __provider.NewCounterVec(name, desc, labels)
}

func NewGauge(name, desc string) Gauge {
	return __provider.NewGauge(name, desc)
}

func NewHistogram(name, desc string) Histogram {
	return __provider.NewHistogram(name, desc)
}

// InitPrometheusMetricsProvider initializes the default metrics provider to
// use Prometheus Go SDK. This function is not thread-safe and should be called
// before any other function in this package.
func InitPrometheusMetricsProvider(namespace, subsystem string) {
	__provider = NewPrometheusMetricsProvider(namespace, subsystem)
}

type MetricsServerConfig struct {
	ServiceName string
	Port        string
}

const DefaultMetricServerPort = ":8080"

func StartMetricsServer(config *MetricsServerConfig) error {
	router, err := dryhttp.NewEchoRouter(dryhttp.EchoRouterConfig{
		ServiceName: config.ServiceName,
	})

	if err != nil {
		return fmt.Errorf("failed to create metrics server: %v", err)
	}

	err = router.ListenAndServe(config.Port)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %v", err)
	}

	return nil
}
