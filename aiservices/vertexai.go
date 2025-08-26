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

type googleVertexAIFastModel struct {
	baseModel model.ToolCallingChatModel
}

var _ Model = &googleVertexAIFastModel{}

func NewGoogleVertexAIFastModel(ctx context.Context, config VertexAIModelConfig) (Model, error) {
	chatModel, err := createVertexAIChatModel(ctx, vertexAIFastModelId, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chat model")
	}

	return &googleVertexAIFastModel{
		baseModel: chatModel,
	}, nil
}

func (g googleVertexAIFastModel) GetProvider() ModelProviderIdentifier {
	return GoogleVertex
}

func (g googleVertexAIFastModel) GetId() string {
	return vertexAIFastModelId
}

type googleVertexAIReasoningModel struct {
	baseModel model.ToolCallingChatModel
}

var _ Model = &googleVertexAIReasoningModel{}

func NewGoogleVertexAIReasoningModel(ctx context.Context, config VertexAIModelConfig) (Model, error) {
	chatModel, err := createVertexAIChatModel(ctx, vertexAIReasoningModelId, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chat model")
	}

	return &googleVertexAIReasoningModel{
		baseModel: chatModel,
	}, nil
}

func (g googleVertexAIReasoningModel) GetProvider() ModelProviderIdentifier {
	return GoogleVertex
}

func (g googleVertexAIReasoningModel) GetId() string {
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
