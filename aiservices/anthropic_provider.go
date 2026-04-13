package aiservices

import "strings"

type anthropicModelProvider struct {
	config AnthropicModelConfig
}

var _ LLMProvider = &anthropicModelProvider{}

// NewAnthropicModelProvider creates a new LLMProvider backed by Anthropic Claude models.
// Set config.UseBedrock to true to route requests through AWS Bedrock; otherwise the
// direct Anthropic API is used.
func NewAnthropicModelProvider(config AnthropicModelConfig) (LLMProvider, error) {
	return &anthropicModelProvider{config: config}, nil
}

func (p *anthropicModelProvider) GetID() ModelProviderIdentifier {
	return Anthropic
}

// GetFastModel returns Claude Haiku — optimized for speed.
// When UseBedrock is true the Bedrock model ID is used; otherwise the direct API model ID.
func (p *anthropicModelProvider) GetFastModel() (LLM, error) {
	modelId := anthropicFastModelId
	if p.config.UseBedrock {
		modelId = bedrockFastModelId
	}
	return newAnthropicChatModel(modelId, p.config)
}

// GetReasoningModel returns Claude Sonnet 4.6 — optimized for reasoning.
// When UseBedrock is true the Bedrock model ID is used; otherwise the direct API model ID.
func (p *anthropicModelProvider) GetReasoningModel() (LLM, error) {
	modelId := anthropicReasoningModelId
	if p.config.UseBedrock {
		modelId = bedrockReasoningModelId
	}
	return newAnthropicChatModel(modelId, p.config)
}

// GetModelByID returns a Claude model by its provider-specific model ID.
func (p *anthropicModelProvider) GetModelByID(id string) (LLM, error) {
	if strings.TrimSpace(id) == "" {
		return nil, NewInvalidConfigError(Anthropic, "model ID cannot be empty")
	}
	return newAnthropicChatModel(id, p.config)
}
