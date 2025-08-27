package aiservices

import (
	"context"

	"github.com/pkg/errors"
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
		return nil, errors.Wrap(err, "failed to create vertex ai fast model")
	}
	return fastModel, nil
}

func (g googleVertexAIModelProvider) GetReasoningModel() (Model, error) {
	reasoningModel, err := newVertexAIChatModel(context.Background(), vertexAIReasoningModelId, g.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vertex ai reasoning model")
	}
	return reasoningModel, nil
}

func (g googleVertexAIModelProvider) GetModelByID(s string) (Model, error) {
	customModel, err := newVertexAIChatModel(context.Background(), s, g.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create vertex ai model id: %s", s)
	}
	return customModel, nil
}
