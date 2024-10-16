package obs

import "testing"

func TestDefaultMetricsProvider(t *testing.T) {
	_ = NewCounter("test", "test")
	_ = NewGauge("test", "test")
	_ = NewHistogram("test", "test")
}
