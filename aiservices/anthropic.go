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
	// Bedrock model IDs include the anthropic. prefix and -v1:0 version suffix
	// required by the Bedrock model catalog. These differ from the direct API IDs.
	// Docs: https://docs.aws.amazon.com/bedrock/latest/userguide/models-supported.html
	// https://models.dev/?search=anthropic
	bedrockFastModelId      = "anthropic.claude-haiku-4-5-20251001-v1:0"
	bedrockReasoningModelId = "anthropic.claude-sonnet-4-6"

	// Direct Anthropic API model IDs.
	// https://models.dev/?search=anthropic
	anthropicFastModelId      = "claude-haiku-4-5"
	anthropicReasoningModelId = "claude-sonnet-4-6"

	// anthropicDefaultMaxTokens is the default max tokens for Claude models.
	// Claude requires MaxTokens to be explicitly set.
	anthropicDefaultMaxTokens = 8192
)

// AnthropicModelConfig holds configuration for Anthropic Claude models.
//
// When UseBedrock is true, the model is served via AWS Bedrock. Credentials are
// resolved from the explicit fields first, then the AWS default credential chain
// (env vars → ~/.aws/credentials → IAM instance role → ECS/EKS task role).
//
// When UseBedrock is false, the model is accessed via the direct Anthropic API
// using APIKey (and optionally BaseURL for custom endpoints or proxies).
type AnthropicModelConfig struct {
	// UseBedrock switches the backend to AWS Bedrock.
	UseBedrock bool

	// Bedrock-specific fields (used when UseBedrock is true).
	// Region is required; the rest are optional and fall back to the credential chain.
	Region          string
	AccessKey       *string
	SecretAccessKey *string
	SessionToken    *string
	Profile         *string

	// Direct API fields (used when UseBedrock is false).
	APIKey  string
	BaseURL *string
}

type anthropicModel struct {
	baseModel model.ToolCallingChatModel
	modelId   string
}

var _ LLM = &anthropicModel{}
var _ Model = &anthropicModel{}

// Docs: https://github.com/cloudwego/eino-ext/tree/main/components/model/claude#readme
func newAnthropicChatModel(modelId string, config AnthropicModelConfig, enableThinking bool) (LLM, error) {
	claudeConfig := &claudemodel.Config{
		Model:     modelId,
		MaxTokens: anthropicDefaultMaxTokens,
	}

	if config.UseBedrock {
		if config.Region == "" {
			return nil, NewInvalidConfigError(Anthropic, "region is required when using the Bedrock backend")
		}
		claudeConfig.ByBedrock = true
		claudeConfig.Region = config.Region
		if config.AccessKey != nil {
			claudeConfig.AccessKey = *config.AccessKey
		}
		if config.SecretAccessKey != nil {
			claudeConfig.SecretAccessKey = *config.SecretAccessKey
		}
		if config.SessionToken != nil {
			claudeConfig.SessionToken = *config.SessionToken
		}
		if config.Profile != nil {
			claudeConfig.Profile = *config.Profile
		}
	} else {
		if config.APIKey == "" {
			return nil, NewInvalidConfigError(Anthropic, "API key is required when using the direct Anthropic backend")
		}
		claudeConfig.APIKey = config.APIKey
		claudeConfig.BaseURL = config.BaseURL
	}

	// Enable thinking for reasoning models
	if enableThinking {
		claudeConfig.Thinking = &claudemodel.Thinking{
			Enable:       enableThinking,
			BudgetTokens: 1024,
		}
	}

	chatModel, err := claudemodel.NewChatModel(context.Background(), claudeConfig)
	if err != nil {
		err = errors.Wrap(err, "failed to create Anthropic chat model")
		return nil, NewAuthenticationError(Anthropic, err.Error())
	}

	return &anthropicModel{
		baseModel: chatModel,
		modelId:   modelId,
	}, nil
}

func (m *anthropicModel) GetProviderID() ModelProviderIdentifier {
	return Anthropic
}

func (m *anthropicModel) GetId() string {
	return m.modelId
}

func (m *anthropicModel) GenerateSingle(ctx context.Context, req LLMGenerationRequest, opts ...inferenceOptionFn) (string, error) {
	generateOptions := modelInferenceOptionsToEinoModelOptions(opts)

	metricAiServicesLlmGenerationTotal.WithLabels(map[string]string{
		"provider": string(Anthropic),
		"model":    m.GetId(),
	}).Inc()

	if req.SystemPrompt == "" {
		return "", NewInvalidConfigError(Anthropic, "system prompt is required")
	}

	if req.UserPrompt == "" {
		return "", NewInvalidRequestError(Anthropic, m.GetId(), "user prompt is required")
	}

	modifiedReq, err := guardedPrompt(req)
	if err != nil {
		return "", NewInvalidRequestError(Anthropic, m.GetId(), "failed to harden prompt: "+err.Error())
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
			"provider": string(Anthropic),
			"model":    m.GetId(),
		}).Inc()

		if strings.Contains(strings.ToLower(err.Error()), "exceeds the maximum number of tokens allowed") {
			return "", NewTokenLimitError(Anthropic, m.GetId(), err.Error())
		}

		err = errors.Wrap(err, "error generating response from Anthropic LLM")
		return "", NewModelUnavailableError(Anthropic, m.GetId(), err.Error())
	}

	return flattenResponseMessage(response), nil
}
