package aiservices

import (
	"context"

	"cloud.google.com/go/auth/credentials"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
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
	ResponseSchema  *openapi3.Schema
}

type googleVertexAIModel struct {
	baseModel model.ToolCallingChatModel
	modelId   string
}

var _ LLM = &googleVertexAIModel{}
var _ Model = &googleVertexAIModel{}

func newVertexAIChatModel(modelId string, config VertexAIModelConfig) (LLM, error) {
	if config.Project == "" {
		return nil, NewInvalidConfigError(GoogleVertex, "project is required for Vertex AI model")
	}

	if config.Location == "" {
		return nil, NewInvalidConfigError(GoogleVertex, "location is required for Vertex AI model")
	}

	// Load credentials from a service account key file
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		CredentialsFile: config.CredentialsFile,
		Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
	})

	if err != nil {
		err := errors.Wrap(err, "failed to load credentials for vertex ai model")
		return nil, NewAuthenticationError(GoogleVertex, err.Error())
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		Project:     config.Project,
		Location:    config.Location,
		Backend:     genai.BackendVertexAI,
		Credentials: creds,
	})

	if err != nil {
		err := errors.Wrap(err, "failed to create gemini client")
		return nil, NewAuthenticationError(GoogleVertex, err.Error())
	}

	// Create and configure ChatModel
	chatModel, err := gemini.NewChatModel(context.Background(), &gemini.Config{
		Model:          modelId,
		Client:         client,
		ResponseSchema: config.ResponseSchema,
	})

	if err != nil {
		err := errors.Wrap(err, "failed to create gemini chat model")
		return nil, NewAuthenticationError(GoogleVertex, err.Error())
	}

	return &googleVertexAIModel{
		baseModel: chatModel,
		modelId:   modelId,
	}, nil
}

func (g googleVertexAIModel) GetProvider() ModelProviderIdentifier {
	return GoogleVertex
}

func (g googleVertexAIModel) GetId() string {
	return g.modelId
}

func (g googleVertexAIModel) GenerateSingle(ctx context.Context, input string, opts ...inferenceOptionFn) (string, error) {
	generateOptions := modelInferenceOptionsToEinoModelOptions(opts)

	response, err := g.baseModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: input,
		},
	}, generateOptions...)

	if err != nil {
		err := errors.Wrap(err, "error generating response from Eino Vertex AI LLM")
		return "", NewModelUnavailableError(GoogleVertex, g.GetId(), err.Error())
	}

	return flattenResponseMessage(response), nil
}
