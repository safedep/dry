package aiservices

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of error that occurred.
type ErrorType string

const (
	// Model-related errors
	ErrorTypeRateLimit        ErrorType = "rate_limit"
	ErrorTypeInvalidConfig    ErrorType = "invalid_config"
	ErrorTypeAuthentication   ErrorType = "authentication"
	ErrorTypeModelUnavailable ErrorType = "model_unavailable"
	ErrorTypeInvalidRequest   ErrorType = "invalid_request"
	ErrorTypeTokenLimit       ErrorType = "token_limit"
	ErrorTypeContentFilter    ErrorType = "content_filter"

	// Agent-related errors
	ErrorTypeAgentExecution ErrorType = "agent_execution"
	ErrorTypeToolExecution  ErrorType = "tool_execution"
	ErrorTypeMemoryFailure  ErrorType = "memory_failure"
	ErrorTypeSessionInvalid ErrorType = "session_invalid"

	// Network-related errors
	ErrorTypeTimeout     ErrorType = "timeout"
	ErrorTypeNetworkFail ErrorType = "network_failure"
	ErrorTypeUnknown     ErrorType = "unknown"
)

// ModelError represents an error from AI model operations.
type ModelError struct {
	Type      ErrorType
	Provider  ModelProviderIdentifier
	ModelID   string
	Message   string
	Retryable bool
	Cause     error
}

func (e *ModelError) Error() string {
	if e.Provider != "" && e.ModelID != "" {
		return fmt.Sprintf("model error [%s/%s]: %s (%s)",
			e.Provider, e.ModelID, e.Message, e.Type)
	}

	if e.Provider != "" {
		return fmt.Sprintf("model error [%s]: %s (%s)",
			e.Provider, e.Message, e.Type)
	}

	return fmt.Sprintf("model error: %s (%s)", e.Message, e.Type)
}

func (e *ModelError) Unwrap() error {
	return e.Cause
}

func (e *ModelError) IsRetryable() bool {
	return e.Retryable
}

// AgentError represents an error from agent operations.
type AgentError struct {
	Type      ErrorType
	SessionID string
	Message   string
	Cause     error
}

func (e *AgentError) Error() string {
	if e.SessionID != "" {
		return fmt.Sprintf("agent error [%s]: %s (%s)", e.SessionID, e.Message, e.Type)
	}

	return fmt.Sprintf("agent error: %s (%s)", e.Message, e.Type)
}

func (e *AgentError) Unwrap() error {
	return e.Cause
}

// NewRateLimitError creates a rate limit error.
func NewRateLimitError(provider ModelProviderIdentifier, modelID, message string) *ModelError {
	return &ModelError{
		Type:      ErrorTypeRateLimit,
		Provider:  provider,
		ModelID:   modelID,
		Message:   message,
		Retryable: true,
	}
}

// NewInvalidConfigError creates an invalid config error.
func NewInvalidConfigError(provider ModelProviderIdentifier, message string) *ModelError {
	return &ModelError{
		Type:      ErrorTypeInvalidConfig,
		Provider:  provider,
		Message:   message,
		Retryable: false,
	}
}

// NewAuthenticationError creates an authentication error.
func NewAuthenticationError(provider ModelProviderIdentifier, message string) *ModelError {
	return &ModelError{
		Type:      ErrorTypeAuthentication,
		Provider:  provider,
		Message:   message,
		Retryable: false,
	}
}

// NewTokenLimitError creates a token limit exceeded error.
func NewTokenLimitError(provider ModelProviderIdentifier, modelID, message string) *ModelError {
	return &ModelError{
		Type:      ErrorTypeTokenLimit,
		Provider:  provider,
		ModelID:   modelID,
		Message:   message,
		Retryable: false,
	}
}

// NewModelUnavailableError creates a model unavailable error.
func NewModelUnavailableError(provider ModelProviderIdentifier, modelID, message string) *ModelError {
	return &ModelError{
		Type:      ErrorTypeModelUnavailable,
		Provider:  provider,
		ModelID:   modelID,
		Message:   message,
		Retryable: true,
	}
}

// NewAgentExecutionError creates an agent execution error.
func NewAgentExecutionError(sessionID, message string, cause error) *AgentError {
	return &AgentError{
		Type:      ErrorTypeAgentExecution,
		SessionID: sessionID,
		Message:   message,
		Cause:     cause,
	}
}

// NewToolExecutionError creates a tool execution error.
func NewToolExecutionError(sessionID, message string, cause error) *AgentError {
	return &AgentError{
		Type:      ErrorTypeToolExecution,
		SessionID: sessionID,
		Message:   message,
		Cause:     cause,
	}
}

// NewMemoryError creates a memory operation error.
func NewMemoryError(sessionID, message string, cause error) *AgentError {
	return &AgentError{
		Type:      ErrorTypeMemoryFailure,
		SessionID: sessionID,
		Message:   message,
		Cause:     cause,
	}
}

// Error checking helpers

// IsRetryableError checks if an error is retryable.
func IsRetryableError(err error) bool {
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		return modelErr.IsRetryable()
	}

	return false
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		return modelErr.Type == ErrorTypeRateLimit
	}

	return false
}

// IsInvalidConfigError checks if an error is an invalid config error.
func IsInvalidConfigError(err error) bool {
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		return modelErr.Type == ErrorTypeInvalidConfig
	}

	return false
}

// IsAuthenticationError checks if an error is an authentication error.
func IsAuthenticationError(err error) bool {
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		return modelErr.Type == ErrorTypeAuthentication
	}

	return false
}

// IsTokenLimitError checks if an error is a token limit error.
func IsTokenLimitError(err error) bool {
	var modelErr *ModelError
	if errors.As(err, &modelErr) {
		return modelErr.Type == ErrorTypeTokenLimit
	}

	return false
}
