package http

import (
	"fmt"
	"os"
)

type MetricsServerConfig struct {
	ServiceName string
	Address     string
}

func DefaultMetricsServerConfig(serviceName string) MetricsServerConfig {
	address := ":8080"
	if addressEnv := os.Getenv("METRICS_SERVER_ADDRESS"); addressEnv != "" {
		address = addressEnv
	}

	return MetricsServerConfig{
		ServiceName: serviceName,
		Address:     address,
	}
}

// StartMetricsServer starts an HTTP server that exposes the
// metrics endpoint. Exposing Prometheus metrics.
func StartMetricsServer(config MetricsServerConfig) error {
	router, err := NewEchoRouter(EchoRouterConfig{
		ServiceName: config.ServiceName,
	})

	if err != nil {
		return fmt.Errorf("failed to create metrics server: %v", err)
	}

	err = router.ListenAndServe(config.Address)
	if err != nil {
		return fmt.Errorf("failed to start metrics server: %v", err)
	}

	return nil
}
