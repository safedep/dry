package aiservices

import (
	"context"
	"strings"
)

type googleVertexAIModelProvider struct {
	config VertexAIModelConfig
}

var _ ModelProvider = &googleVertexAIModelProvider{}

func NewGoogleVertexAIModelProvider(config VertexAIModelConfig) (ModelProvider, error) {
	return &googleVertexAIModelProvider{config: config}, nil
}

func (g googleVertexAIModelProvider) GetFastModel() (Model, error) {
	fastModel, err := newVertexAIChatModel(context.Background(), vertexAIFastModelId, g.config)
	if err != nil {
		return nil, err
	}
	return fastModel, nil
}

func (g googleVertexAIModelProvider) GetReasoningModel() (Model, error) {
	reasoningModel, err := newVertexAIChatModel(context.Background(), vertexAIReasoningModelId, g.config)
	if err != nil {
		return nil, err
	}
	return reasoningModel, nil
}

func (g googleVertexAIModelProvider) GetModelByID(s string) (Model, error) {
	if strings.TrimSpace(s) == "" {
		return nil, NewInvalidConfigError(GoogleVertex, "model ID cannot be empty")
	}

	customModel, err := newVertexAIChatModel(context.Background(), s, g.config)
	if err != nil {
		return nil, err
	}
	return customModel, nil
}
