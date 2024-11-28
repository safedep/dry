package async

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRpcTopicName(t *testing.T) {
	cases := []struct {
		name        string
		serviceName string
		methodName  string
		expected    string
	}{
		{
			name:        "simple",
			serviceName: "service",
			methodName:  "method",
			expected:    "service.method",
		},
		{
			name:        "simple with slash",
			serviceName: "/service",
			methodName:  "/method",
			expected:    "service.method",
		},
		{
			name:        "service with multiple slash",
			serviceName: "/service/a/b/c",
			methodName:  "/method",
			expected:    "service.a.b.c.method",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := RpcTopicName(test.serviceName, test.methodName)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestRpcTopicNameFromFullProcedureName(t *testing.T) {
	cases := []struct {
		name              string
		fullProcedureName string
		expected          string
	}{
		{
			name:              "simple",
			fullProcedureName: "/service/method",
			expected:          "service.method",
		},
		{
			name:              "simple with slash",
			fullProcedureName: "/service/method/",
			expected:          "service.method",
		},
		{
			name:              "service with multiple slash",
			fullProcedureName: "/service/a/b/c/method",
			expected:          "service.a.b.c.method",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := RpcTopicNameFromFullProcedureName(test.fullProcedureName)
			assert.Equal(t, test.expected, actual)
		})
	}
}
