package aiservices

import "strings"

type anthropicBedrockModelProvider struct {
	config BedrockModelConfig
}

var _ LLMProvider = &anthropicBedrockModelProvider{}

// NewAnthropicBedrockModelProvider creates a new LLMProvider backed by Anthropic Claude
// models on AWS Bedrock. AWS credentials are resolved via the default credential chain.
func NewAnthropicBedrockModelProvider(config BedrockModelConfig) (LLMProvider, error) {
	return &anthropicBedrockModelProvider{config: config}, nil
}

func (p *anthropicBedrockModelProvider) GetID() ModelProviderIdentifier {
	return AnthropicBedrock
}

// GetFastModel returns Claude Haiku on Bedrock, optimized for speed.
func (p *anthropicBedrockModelProvider) GetFastModel() (LLM, error) {
	return newBedrockChatModel(bedrockFastModelId, p.config)
}

// GetReasoningModel returns Claude Sonnet 4.6 on Bedrock, optimized for reasoning.
func (p *anthropicBedrockModelProvider) GetReasoningModel() (LLM, error) {
	return newBedrockChatModel(bedrockReasoningModelId, p.config)
}

// GetModelByID returns a Claude model on Bedrock by its Bedrock model ID.
func (p *anthropicBedrockModelProvider) GetModelByID(id string) (LLM, error) {
	if strings.TrimSpace(id) == "" {
		return nil, NewInvalidConfigError(AnthropicBedrock, "model ID cannot be empty")
	}
	return newBedrockChatModel(id, p.config)
}
