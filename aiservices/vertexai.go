package aiservices

import (
	"context"

	"cloud.google.com/go/auth/credentials"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"github.com/pkg/errors"
	"google.golang.org/genai"
)

const (
	vertexAIFastModelId      = "gemini-2.5-flash"
	vertexAIReasoningModelId = "gemini-2.5-pro"
)

type VertexAIModelConfig struct {
	Project         string
	Location        string
	CredentialsFile string
}

type GoogleVertexAIFastModel struct {
	baseModel model.ToolCallingChatModel
	config    VertexAIModelConfig
}

var _ Model = &GoogleVertexAIFastModel{}

func NewGoogleVertexAIFastModel(ctx context.Context, config VertexAIModelConfig) (*GoogleVertexAIFastModel, error) {
	chatModel, err := createVertexAIChatModel(ctx, vertexAIFastModelId, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chat model")
	}

	return &GoogleVertexAIFastModel{
		baseModel: chatModel,
		config:    config,
	}, nil
}

func (g GoogleVertexAIFastModel) GetProvider() ModelProviderIdentifier {
	return GoogleVertex
}

func (g GoogleVertexAIFastModel) GetId() string {
	return vertexAIFastModelId
}

type GoogleVertexAIReasoningModel struct {
	baseModel model.ToolCallingChatModel
	config    VertexAIModelConfig
}

var _ Model = &GoogleVertexAIReasoningModel{}

func NewGoogleVertexAIReasoningModel(ctx context.Context, config VertexAIModelConfig) (*GoogleVertexAIReasoningModel, error) {
	chatModel, err := createVertexAIChatModel(ctx, vertexAIReasoningModelId, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chat model")
	}

	return &GoogleVertexAIReasoningModel{
		baseModel: chatModel,
		config:    config,
	}, nil
}

func (g GoogleVertexAIReasoningModel) GetProvider() ModelProviderIdentifier {
	return GoogleVertex
}

func (g GoogleVertexAIReasoningModel) GetId() string {
	return vertexAIReasoningModelId
}

func createVertexAIChatModel(ctx context.Context, modelId string, config VertexAIModelConfig) (model.ToolCallingChatModel, error) {
	if config.Project == "" {
		return nil, errors.New("project is required for Vertex AI model")
	}

	if config.Location == "" {
		return nil, errors.New("location is required for Vertex AI model")
	}

	// Load credentials from a service account key file
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		CredentialsFile: config.CredentialsFile,
		Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to load credentials")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:     config.Project,
		Location:    config.Location,
		Backend:     genai.BackendVertexAI,
		Credentials: creds,
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to create gemini client")
	}

	// Create and configure ChatModel
	chatModel, err := gemini.NewChatModel(context.Background(), &gemini.Config{
		Model:  modelId,
		Client: client,
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to create gemini model")
	}

	return chatModel, nil
}

type GoogleVertexAIModelProvider struct {
	config VertexAIModelConfig
}

func NewGoogleVertexAIModelProvider(config VertexAIModelConfig) *GoogleVertexAIModelProvider {
	return &GoogleVertexAIModelProvider{config: config}
}

func (g GoogleVertexAIModelProvider) GetFastModel() (Model, error) {
	fastModel, err := NewGoogleVertexAIFastModel(context.Background(), g.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vertex ai fast model")
	}
	return fastModel, nil
}

func (g GoogleVertexAIModelProvider) GetReasoningModel() (Model, error) {
	reasoningModel, err := NewGoogleVertexAIReasoningModel(context.Background(), g.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create vertex ai fast model")
	}
	return reasoningModel, nil
}

func (g GoogleVertexAIModelProvider) GetModelByID(s string) (Model, error) {
	switch s {
	case vertexAIFastModelId:
		return g.GetFastModel()
	case vertexAIReasoningModelId:
		return g.GetReasoningModel()
	default:
		return nil, errors.New("invalid model ID")
	}
}
