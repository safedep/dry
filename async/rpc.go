package async

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
)

func RpcTopicName(serviceName, methodName string) string {
	fixer := func(r string) string {
		if len(r) == 0 {
			return ""
		}

		r = strings.TrimPrefix(r, "/")
		r = strings.TrimSuffix(r, "/")

		return strings.ReplaceAll(r, "/", ".")
	}

	return fixer(serviceName) + "." + fixer(methodName)
}

// RpcNamespacedTopicName returns the topic name for a namespaced RPC procedure.
// If the namespace is empty, the behaviour is exactly same as RpcTopicName.
func RpcNamespacedTopicName(serviceName, methodName, namespace string) string {
	return rpcNamespacedTopicName(RpcTopicName(serviceName, methodName), namespace)
}

// RpcNamespacedRequestTopicName creates a namespaced request topic name
// This is a convention driven approach to create a request topic name from a gRPC
// service and method name, along with a namespace. This is required when we are
// in streaming mode and need two streams: one for requests and one for responses.
func RpcNamespacedRequestTopicName(serviceName, methodName, namespace string) string {
	return rpcRequestTopicName(RpcNamespacedTopicName(serviceName, methodName, namespace))
}

// RpcNamespacedResponseTopicName creates a namespaced response topic namespace
// This is a convention driven approach to create a response topic name from a gRPC
// service and method name, along with a namespace. This is required when we are
// in streaming mode and need two streams: one for requests and one for responses.
func RpcNamespacedResponseTopicName(serviceName, methodName, namespace string) string {
	return rpcResponseTopicName(RpcNamespacedTopicName(serviceName, methodName, namespace))
}

// RpcGetServiceAndMethodFromFullProcedureName extracts service and method names
// from a full procedure name using our conventions.
func RpcGetServiceAndMethodFromFullProcedureName(fullProcedureName string) (string, string, error) {
	return rpcGetServiceAndMethodFromFullProcedureName(fullProcedureName)
}

// RpcTopicNameFromFullProcedureName generates a topic name from a full procedure name
// using our conventions. Originally created for gRPC, but generic enough to be used
// for other RPC systems as well.
func RpcTopicNameFromFullProcedureName(fullProcedureName string) string {
	serviceName, methodName, err := rpcGetServiceAndMethodFromFullProcedureName(fullProcedureName)
	if err != nil {
		return ""
	}

	return RpcTopicName(serviceName, methodName)
}

type RpcCallOptions struct {
	Extra   MessageExtra
	Timeout time.Duration
}

// RpcInvoke invokes an RPC procedure over an AsyncRpcClient using our conventions.
func RpcInvoke[I, O proto.Message](ctx context.Context, client AsyncRpcClient,
	fullProcedureName string, input I, output O, options RpcCallOptions) error {
	topicName := RpcTopicNameFromFullProcedureName(fullProcedureName)
	if len(topicName) == 0 {
		return fmt.Errorf("invalid full procedure name: %s", fullProcedureName)
	}

	return rpcInvokeWithTopicName(ctx, client, topicName, input, output, options)
}

// RpcInvokeWithNamespace invokes an RPC procedure over an AsyncRpcClient using a namespaced
// topic name. If the namespace is empty, the behavior is exactly same as RpcInvoke.
func RpcInvokeWithNamespace[I, O proto.Message](ctx context.Context, client AsyncRpcClient,
	namespace, fullProcedureName string, input I, output O, options RpcCallOptions) error {
	topicName := RpcTopicNameFromFullProcedureName(fullProcedureName)
	if len(topicName) == 0 {
		return fmt.Errorf("invalid full procedure name: %s", fullProcedureName)
	}

	topicName = rpcNamespacedTopicName(topicName, namespace)
	return rpcInvokeWithTopicName(ctx, client, topicName, input, output, options)
}

func rpcInvokeWithTopicName[I, O proto.Message](ctx context.Context, client AsyncRpcClient,
	topicName string, input I, output O, options RpcCallOptions) error {
	data, err := proto.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to serialize protobuf message: %w", err)
	}

	resp, err := client.Call(ctx, topicName, data, options.Timeout)
	if err != nil {
		return fmt.Errorf("rpc call failed: %w", err)
	}

	if err := proto.Unmarshal(resp, output); err != nil {
		return fmt.Errorf("failed to deserialize protobuf message: %w", err)
	}

	return nil
}

func rpcNamespacedTopicName(topicName, namespace string) string {
	if len(namespace) == 0 {
		return topicName
	}

	return fmt.Sprintf("namespaced.%s.%s", namespace, topicName)
}

func rpcGetServiceAndMethodFromFullProcedureName(fullProcedureName string) (string, string, error) {
	if len(fullProcedureName) == 0 {
		return "", "", fmt.Errorf("full procedure name cannot be empty")
	}

	if fullProcedureName[0] == '/' {
		fullProcedureName = fullProcedureName[1:]
	}

	parts := strings.SplitN(fullProcedureName, "/", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid full procedure name: %s", fullProcedureName)
	}

	return parts[0], parts[1], nil
}

func rpcRequestTopicName(topicName string) string {
	return fmt.Sprintf("%s.request", topicName)
}

func rpcResponseTopicName(topicName string) string {
	return fmt.Sprintf("%s.response", topicName)
}
