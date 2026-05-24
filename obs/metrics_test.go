package obs

import "testing"

func TestDefaultMetricsProvider(t *testing.T) {
	_ = NewCounter("test", "test")
	_ = NewGauge("test", "test")
	_ = NewHistogram("test", "test")
}

func TestDefaultMetricsProviderGaugeVec(t *testing.T) {
	gv := NewGaugeVec("test", "test", []string{"label1"})
	gv.WithLabels(map[string]string{"label1": "1"}).Set(1)
}
