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

func RpcTopicNameFromFullProcedureName(fullProcedureName string) string {
	if len(fullProcedureName) == 0 {
		return ""
	}

	if fullProcedureName[0] == '/' {
		fullProcedureName = fullProcedureName[1:]
	}

	parts := strings.SplitN(fullProcedureName, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	return RpcTopicName(parts[0], parts[1])
}

type RpcCallOptions struct {
	Extra   MessageExtra
	Timeout time.Duration
}

// Invoke an RPC procedure over an AsyncRpcClient using our conventions.
func RpcInvoke[I, O proto.Message](ctx context.Context, client AsyncRpcClient,
	fullProcedureName string, input I, output O, options RpcCallOptions) error {
	topicName := RpcTopicNameFromFullProcedureName(fullProcedureName)
	if len(topicName) == 0 {
		return fmt.Errorf("invalid full procedure name: %s", fullProcedureName)
	}

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
