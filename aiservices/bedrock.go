package aiservices

import (
	"context"
	"strings"

	claudemodel "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/pkg/errors"
)

const (
	// bedrockFastModelId is the model ID for Claude Haiku on AWS Bedrock.
	// Haiku is optimized for speed and cost-efficiency.
	bedrockFastModelId = "anthropic.claude-haiku-4-5"

	// bedrockReasoningModelId is the model ID for Claude Sonnet 4.6 on AWS Bedrock.
	// Sonnet 4.6 is optimized for reasoning and complex tasks.
	bedrockReasoningModelId = "anthropic.claude-sonnet-4-6"

	// bedrockDefaultMaxTokens is the default max tokens for Bedrock models.
	// Claude on Bedrock requires MaxTokens to be explicitly set.
	bedrockDefaultMaxTokens = 8192
)

// BedrockModelConfig holds configuration for Anthropic Claude models on AWS Bedrock.
//
// Credentials are resolved in order:
//  1. Explicit AccessKey + SecretAccessKey (+ optional SessionToken) if provided
//  2. Profile if set (reads from ~/.aws/credentials)
//  3. AWS default credential chain: env vars → ~/.aws/credentials → IAM instance role → ECS/EKS task role
type BedrockModelConfig struct {
	Region string

	// AccessKey and SecretAccessKey are optional explicit AWS credentials.
	// When empty the AWS default credential chain is used.
	AccessKey       string
	SecretAccessKey string

	// SessionToken is required when using temporary credentials (e.g. STS AssumeRole).
	SessionToken string

	// Profile selects a named profile from ~/.aws/credentials.
	// Ignored when AccessKey and SecretAccessKey are provided.
	Profile string
}

type anthropicBedrockModel struct {
	baseModel model.ToolCallingChatModel
	modelId   string
}

var _ LLM = &anthropicBedrockModel{}
var _ Model = &anthropicBedrockModel{}

// Docs: https://github.com/cloudwego/eino-ext/tree/main/components/model/claude#readme
func newBedrockChatModel(modelId string, config BedrockModelConfig) (LLM, error) {
	if config.Region == "" {
		return nil, NewInvalidConfigError(AnthropicBedrock, "region is required for Anthropic Bedrock model")
	}

	// ByBedrock: true with empty AccessKey/SecretAccessKey causes the AWS SDK
	// to fall back to the default credential chain (env vars → ~/.aws/credentials → IAM role).
	chatModel, err := claudemodel.NewChatModel(context.Background(), &claudemodel.Config{
		ByBedrock:       true,
		Region:          config.Region,
		AccessKey:       config.AccessKey,
		SecretAccessKey: config.SecretAccessKey,
		SessionToken:    config.SessionToken,
		Profile:         config.Profile,
		Model:           modelId,
		MaxTokens:       bedrockDefaultMaxTokens,
	})
	if err != nil {
		err = errors.Wrap(err, "failed to create Anthropic Bedrock chat model")
		return nil, NewAuthenticationError(AnthropicBedrock, err.Error())
	}

	return &anthropicBedrockModel{
		baseModel: chatModel,
		modelId:   modelId,
	}, nil
}

func (m *anthropicBedrockModel) GetProviderID() ModelProviderIdentifier {
	return AnthropicBedrock
}

func (m *anthropicBedrockModel) GetId() string {
	return m.modelId
}

func (m *anthropicBedrockModel) GenerateSingle(ctx context.Context, req LLMGenerationRequest, opts ...inferenceOptionFn) (string, error) {
	generateOptions := modelInferenceOptionsToEinoModelOptions(opts)

	metricAiServicesLlmGenerationTotal.WithLabels(map[string]string{
		"provider": string(AnthropicBedrock),
		"model":    m.GetId(),
	}).Inc()

	if req.SystemPrompt == "" {
		return "", NewInvalidConfigError(AnthropicBedrock, "system prompt is required for Anthropic Bedrock model")
	}

	if req.UserPrompt == "" {
		return "", NewInvalidRequestError(AnthropicBedrock, m.GetId(), "user prompt is required for Anthropic Bedrock model")
	}

	modifiedReq, err := guardedPrompt(req)
	if err != nil {
		return "", NewInvalidRequestError(AnthropicBedrock, m.GetId(), "failed to harden prompt: "+err.Error())
	}

	response, err := m.baseModel.Generate(ctx, []*schema.Message{
		{
			Role:    schema.System,
			Content: modifiedReq.systemPrompt,
		},
		{
			Role:    schema.User,
			Content: modifiedReq.userPrompt,
		},
	}, generateOptions...)

	if err != nil {
		metricAiServicesLlmGenerationErrors.WithLabels(map[string]string{
			"provider": string(AnthropicBedrock),
			"model":    m.GetId(),
		}).Inc()

		if strings.Contains(err.Error(), "exceeds the maximum number of tokens allowed") {
			return "", NewTokenLimitError(AnthropicBedrock, m.GetId(), err.Error())
		}

		err = errors.Wrap(err, "error generating response from Eino Anthropic Bedrock LLM")
		return "", NewModelUnavailableError(AnthropicBedrock, m.GetId(), err.Error())
	}

	return flattenResponseMessage(response), nil
}
