package async

import (
	"context"
	"errors"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"
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

func TestNamespacedRpcTopicName(t *testing.T) {
	cases := []struct {
		name        string
		serviceName string
		methodName  string
		namespace   string
		expected    string
	}{
		{
			name:        "simple",
			serviceName: "service",
			methodName:  "method",
			namespace:   "namespace",
			expected:    "namespaced.namespace.service.method",
		},
		{
			name:        "simple with slash",
			serviceName: "/service",
			methodName:  "/method",
			namespace:   "namespace",
			expected:    "namespaced.namespace.service.method",
		},
		{
			name:        "service with multiple slash",
			serviceName: "/service/a/b/c",
			methodName:  "/method",
			namespace:   "namespace",
			expected:    "namespaced.namespace.service.a.b.c.method",
		},
		{
			name:        "empty namespace",
			serviceName: "service",
			methodName:  "method",
			namespace:   "",
			expected:    "service.method",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := RpcNamespacedTopicName(test.serviceName, test.methodName, test.namespace)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestRpcInvokeWithNamespace(t *testing.T) {
	cases := []struct {
		name              string
		setup             func(t *testing.T, mockClient *MockAsyncRpcClient)
		namespace         string
		fullProcedureName string
		input             proto.Message
		output            proto.Message
		expected          error
	}{
		{
			name: "namespaced valid call",
			setup: func(t *testing.T, mockClient *MockAsyncRpcClient) {
				mockClient.On("Call", mock.Anything, "namespaced.namespace.service.a.b.method", mock.Anything, mock.Anything).Return(nil, nil)
			},
			namespace:         "namespace",
			fullProcedureName: "service.a.b/method",
			input:             &packagev1.PackageManifest{},
			output:            &packagev1.PackageManifest{},
			expected:          nil,
		},
		{
			name:              "namespaced invalid call",
			setup:             func(t *testing.T, mockClient *MockAsyncRpcClient) {},
			namespace:         "namespace",
			fullProcedureName: "service.method",
			input:             &packagev1.PackageManifest{},
			output:            &packagev1.PackageManifest{},
			expected:          errors.New("invalid full procedure name: service.method"),
		},
		{
			name: "namespace is empty",
			setup: func(t *testing.T, mockClient *MockAsyncRpcClient) {
				mockClient.On("Call", mock.Anything, "service.a.b.method", mock.Anything, mock.Anything).Return(nil, nil)
			},
			namespace:         "",
			fullProcedureName: "service.a.b/method",
			input:             &packagev1.PackageManifest{},
			output:            &packagev1.PackageManifest{},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			mockClient := NewMockAsyncRpcClient(t)
			test.setup(t, mockClient)

			err := RpcInvokeWithNamespace(context.Background(),
				mockClient,
				test.namespace,
				test.fullProcedureName,
				test.input,
				test.output,
				RpcCallOptions{},
			)

			if test.expected != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRpcNamespacedRequestTopicName(t *testing.T) {
	cases := []struct {
		name        string
		namespace   string
		serviceName string
		methodName  string
		expected    string
	}{
		{
			name:        "simple",
			namespace:   "namespace",
			serviceName: "service",
			methodName:  "method",
			expected:    "namespaced.namespace.service.method.request",
		},
		{
			name:        "simple with slash",
			namespace:   "namespace",
			serviceName: "/service",
			methodName:  "/method",
			expected:    "namespaced.namespace.service.method.request",
		},
		{
			name:        "service with multiple slash",
			namespace:   "namespace",
			serviceName: "/service/a/b/c",
			methodName:  "/method",
			expected:    "namespaced.namespace.service.a.b.c.method.request",
		},
		{
			name:        "empty namespace",
			namespace:   "",
			serviceName: "service",
			methodName:  "method",
			expected:    "service.method.request",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := RpcNamespacedRequestTopicName(test.serviceName, test.methodName, test.namespace)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestRpcNamespacedResponseTopicName(t *testing.T) {
	cases := []struct {
		name        string
		namespace   string
		serviceName string
		methodName  string
		expected    string
	}{
		{
			name:        "simple",
			namespace:   "namespace",
			serviceName: "service",
			methodName:  "method",
			expected:    "namespaced.namespace.service.method.response",
		},
		{
			name:        "simple with slash",
			namespace:   "namespace",
			serviceName: "/service",
			methodName:  "/method",
			expected:    "namespaced.namespace.service.method.response",
		},
		{
			name:        "service with multiple slash",
			namespace:   "namespace",
			serviceName: "/service/a/b/c",
			methodName:  "/method",
			expected:    "namespaced.namespace.service.a.b.c.method.response",
		},
		{
			name:        "empty namespace",
			namespace:   "",
			serviceName: "service",
			methodName:  "method",
			expected:    "service.method.response",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := RpcNamespacedResponseTopicName(test.serviceName, test.methodName, test.namespace)
			assert.Equal(t, test.expected, actual)
		})
	}
}
