package agent

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// AgentExecutionContext is the context for executing an agent. This is different from config
// because a single configured agent can be executed multiple times with different execution contexts.
type AgentExecutionContext struct {
	// OnToolCall is a hook that is called before executing a tool. This is immutable and for inspection only.
	// This means, the callback function cannot modify the tool params
	OnToolCall func(ctx context.Context, name, arguments string) error

	// InferenceMessageModifier is a hook that allows modifying the inference messages
	// before they are sent to the model in agentic iterations.
	InferenceMessageModifier func(ctx context.Context, input []*schema.Message) []*schema.Message

	// WithComposeOptions are eino native graph / chain config options, like adding a callback handler.
	// Example: github.com/cloudwego/eino/compose.WithCallback
	WithComposeOptions []compose.Option
}

type AgentExecutionContextOption func(hooks *AgentExecutionContext)

// WithOnToolCall sets the OnToolCall hook for the agent execution context.
func WithOnToolCall(fn func(ctx context.Context, name, arguments string) error) AgentExecutionContextOption {
	return func(hooks *AgentExecutionContext) {
		hooks.OnToolCall = fn
	}
}

// WithOnInferenceMessageModifier sets the InferenceMessageModifier hook for the agent execution context.
func WithOnInferenceMessageModifier(fn func(ctx context.Context,
	input []*schema.Message) []*schema.Message) AgentExecutionContextOption {
	return func(hooks *AgentExecutionContext) {
		hooks.InferenceMessageModifier = fn
	}
}

// WithComposeOptions sets the eino compose options for the agent execution context.
func WithComposeOptions(opts ...compose.Option) AgentExecutionContextOption {
	return func(hooks *AgentExecutionContext) {
		hooks.WithComposeOptions = append(hooks.WithComposeOptions, opts...)
	}
}

// Agent is the interface for implementing single turn agents.
type Agent interface {
	// Execute starts the agent execution within the provided context and session.
	// The input parameter is the initial input to the agent.
	Execute(ctx context.Context, session Session, input string,
		options ...AgentExecutionContextOption) (output string, err error)
}
