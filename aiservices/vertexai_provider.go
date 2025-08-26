package aiservices

import (
	"context"

	"github.com/pkg/errors"
)

type googleVertexAIModelProvider struct {
	config VertexAIModelConfig
}

func NewGoogleVertexAIModelProvider(config VertexAIModelConfig) (ModelProvider, error) {
	return &googleVertexAIModelProvider{config: config}, nil
}

func (g googleVertexAIModelProvider) GetFastModel() (Model, error) {
	fastModel, err := NewGoogleVertexAIFastModel(context.Background(), g.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vertex ai fast model")
	}
	return fastModel, nil
}

func (g googleVertexAIModelProvider) GetReasoningModel() (Model, error) {
	reasoningModel, err := NewGoogleVertexAIReasoningModel(context.Background(), g.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vertex ai fast model")
	}
	return reasoningModel, nil
}

func (g googleVertexAIModelProvider) GetModelByID(s string) (Model, error) {
	switch s {
	case vertexAIFastModelId:
		return g.GetFastModel()
	case vertexAIReasoningModelId:
		return g.GetReasoningModel()
	default:
		return nil, errors.New("invalid model ID")
	}
}
