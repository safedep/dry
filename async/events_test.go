package async

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainEventTopicName(t *testing.T) {
	cases := []struct {
		name        string
		serviceName string
		eventName   string
		expected    string
	}{
		{
			name:        "simple",
			serviceName: "order-service",
			eventName:   "OrderCreated",
			expected:    "events.order-service.OrderCreated",
		},
		{
			name:        "with slashes",
			serviceName: "/safedep.services.foo.v1.FooService",
			eventName:   "PackageAnalyzed",
			expected:    "events.safedep.services.foo.v1.FooService.PackageAnalyzed",
		},
		{
			name:        "service with multiple slash segments",
			serviceName: "/service/a/b/c",
			eventName:   "SomethingHappened",
			expected:    "events.service.a.b.c.SomethingHappened",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := DomainEventTopicName(test.serviceName, test.eventName)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestDomainEventNamespacedTopicName(t *testing.T) {
	cases := []struct {
		name        string
		serviceName string
		eventName   string
		namespace   string
		expected    string
	}{
		{
			name:        "with namespace",
			serviceName: "order-service",
			eventName:   "OrderCreated",
			namespace:   "tenant-123",
			expected:    "namespaced.tenant-123.events.order-service.OrderCreated",
		},
		{
			name:        "empty namespace falls back to base",
			serviceName: "order-service",
			eventName:   "OrderCreated",
			namespace:   "",
			expected:    "events.order-service.OrderCreated",
		},
		{
			name:        "namespaced with slashes in service",
			serviceName: "/safedep.services.foo.v1.FooService",
			eventName:   "PackageAnalyzed",
			namespace:   "prod",
			expected:    "namespaced.prod.events.safedep.services.foo.v1.FooService.PackageAnalyzed",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := DomainEventNamespacedTopicName(test.serviceName, test.eventName, test.namespace)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestDomainEventTopicNameFromFullProcedureName(t *testing.T) {
	cases := []struct {
		name              string
		fullProcedureName string
		expected          string
	}{
		{
			name:              "valid procedure name",
			fullProcedureName: "/safedep.services.malysis.v1.MalwareAnalysisService/PackageAnalyzed",
			expected:          "events.safedep.services.malysis.v1.MalwareAnalysisService.PackageAnalyzed",
		},
		{
			name:              "invalid procedure name",
			fullProcedureName: "invalid",
			expected:          "",
		},
		{
			name:              "empty procedure name",
			fullProcedureName: "",
			expected:          "",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := DomainEventTopicNameFromFullProcedureName(test.fullProcedureName)
			assert.Equal(t, test.expected, actual)
		})
	}
}
