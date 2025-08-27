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

type googleVertexAIModel struct {
	baseModel model.ToolCallingChatModel
	modelId   string
}

var _ Model = &googleVertexAIModel{}


func (g googleVertexAIModel) GetProvider() ModelProviderIdentifier {
	return GoogleVertex
}

func (g googleVertexAIModel) GetId() string {
	return g.modelId
}

func newVertexAIChatModel(ctx context.Context, modelId string, config VertexAIModelConfig) (Model, error) {
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

	return &googleVertexAIModel{
		baseModel: chatModel,
		modelId:  modelId,
	}, nil
}
