package aiservices

import "context"

type ModelProviderIdentifier string

// Aligned with https://models.dev/
const (
	OpenAI       ModelProviderIdentifier = "openai"
	Anthropic    ModelProviderIdentifier = "anthropic"
	Google       ModelProviderIdentifier = "google"
	GoogleVertex ModelProviderIdentifier = "google-vertex"
)

// Model is an interface that all AI models must implement.
type Model interface {
	// GetProvider returns the provider of the model.
	GetProviderID() ModelProviderIdentifier

	// GetId returns the unique identifier of the model.
	// This ID is provider-specific.
	GetId() string
}

// ModelInferenceOptions contains general configuration for a Large Language Model
// inference request.
type ModelInferenceOptions struct {
	Temperature *float32
	TopP        *float32
	MaxTokens   *int
	StopWords   []string
}

type inferenceOptionFn func(*ModelInferenceOptions)

// WithTemperature sets the temperature for the model configuration.
// If not set, the model's default temperature will be used.
func WithTemperature(temperature float32) inferenceOptionFn {
	return func(mc *ModelInferenceOptions) {
		mc.Temperature = &temperature
	}
}

// WithTopP sets the top_p for the model configuration.
func WithTopP(topP float32) inferenceOptionFn {
	return func(mc *ModelInferenceOptions) {
		mc.TopP = &topP
	}
}

// WithMaxTokens sets the max_tokens for the model configuration.
// If not set, the model's default max tokens will be used.
func WithMaxTokens(maxTokens int) inferenceOptionFn {
	return func(mc *ModelInferenceOptions) {
		mc.MaxTokens = &maxTokens
	}
}

// WithStopWords sets the stop words for the model configuration.
// If not set, the model's default stop words will be used.
func WithStopWords(stopWords []string) inferenceOptionFn {
	return func(mc *ModelInferenceOptions) {
		mc.StopWords = stopWords
	}
}

// LLM is an interface that all Large Language Models must implement.
type LLM interface {
	Model

	// GenerateSingle generates a single response from the model given a prompt.
	// The configurationFns are optional functions that can be used to customize
	// the model configuration for this specific request.
	GenerateSingle(context.Context, string, ...inferenceOptionFn) (string, error)
}

// LLMProvider is an interface that all llm providers must implement.
// This is our opinionated abstraction over different AI llm interface providers.
type LLMProvider interface {
	// GetID returns the unique identifier of the provider.
	GetID() ModelProviderIdentifier

	// GetFastModel returns a Model that is optimized for speed.
	GetFastModel() (LLM, error)

	// GetReasoningModel returns a Model that is optimized for reasoning.
	GetReasoningModel() (LLM, error)

	// GetModelByID returns a Model by its unique identifier.
	GetModelByID(string) (LLM, error)
}
