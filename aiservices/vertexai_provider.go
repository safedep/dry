package aiservices

import (
	"strings"
)

type googleVertexAIModelProvider struct {
	config VertexAIModelConfig
}

var _ LLMProvider = &googleVertexAIModelProvider{}

func NewGoogleVertexAIModelProvider(config VertexAIModelConfig) (LLMProvider, error) {
	return &googleVertexAIModelProvider{config: config}, nil
}

func (g googleVertexAIModelProvider) GetID() ModelProviderIdentifier {
	return GoogleVertex
}

func (g googleVertexAIModelProvider) GetFastModel() (LLM, error) {
	fastModel, err := newVertexAIChatModel(vertexAIFastModelId, g.config)
	if err != nil {
		return nil, err
	}
	return fastModel, nil
}

func (g googleVertexAIModelProvider) GetReasoningModel() (LLM, error) {
	reasoningModel, err := newVertexAIChatModel(vertexAIReasoningModelId, g.config)
	if err != nil {
		return nil, err
	}
	return reasoningModel, nil
}

func (g googleVertexAIModelProvider) GetModelByID(s string) (LLM, error) {
	if strings.TrimSpace(s) == "" {
		return nil, NewInvalidConfigError(GoogleVertex, "model ID cannot be empty")
	}

	customModel, err := newVertexAIChatModel(s, g.config)
	if err != nil {
		return nil, err
	}
	return customModel, nil
}
