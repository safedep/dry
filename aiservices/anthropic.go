package aiservices

import (
	"context"
	"encoding/json"
	"strings"

	claudemodel "github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/pkg/errors"
)

const (
	// Bedrock model IDs include the anthropic. prefix and -v1:0 version suffix
	// required by the Bedrock model catalog. These differ from the direct API IDs.
	// https://docs.aws.amazon.com/bedrock/latest/userguide/inference-profiles-support.html
	bedrockFastModelId      = "global.anthropic.claude-haiku-4-5-20251001-v1:0"
	bedrockReasoningModelId = "global.anthropic.claude-sonnet-4-6"

	// Direct Anthropic API model IDs.
	// https://models.dev/?search=anthropic
	anthropicFastModelId      = "claude-haiku-4-5"
	anthropicReasoningModelId = "claude-sonnet-4-6"

	// anthropicDefaultMaxTokens is the default max tokens for Claude models.
	// Claude requires MaxTokens to be explicitly set.
	anthropicDefaultMaxTokens = 8192

	// anthropicThinkingBudgetTokens is the budget for thinking models.
	anthropicThinkingBudgetTokens = 1024
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

	// MaxTokens caps the response length. Defaults to anthropicDefaultMaxTokens when nil.
	MaxTokens *int

	// Enable thinking, defaults to false
	// Requirements:
	//   - Temperature must be set to 1
	ThinkingEnabled bool

	// ThinkingBudgetTokens sets the thinking token budget for reasoning models.
	// Defaults to anthropicThinkingBudgetTokens when nil.
	ThinkingBudgetTokens *int

	// ResponseSchema constrains Claude's response to a JSON schema via output_config.format.
	// When set, the model will return structured JSON conforming to this schema.
	ResponseSchema *openapi3.Schema
}

type anthropicModel struct {
	baseModel model.ToolCallingChatModel
	modelId   string
}

var _ LLM = &anthropicModel{}
var _ Model = &anthropicModel{}

// Docs: https://github.com/cloudwego/eino-ext/tree/main/components/model/claude#readme
func newAnthropicChatModel(modelId string, config AnthropicModelConfig) (LLM, error) {
	maxTokens := anthropicDefaultMaxTokens
	if config.MaxTokens != nil {
		maxTokens = *config.MaxTokens
	}

	claudeConfig := &claudemodel.Config{
		Model:     modelId,
		MaxTokens: maxTokens,
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

	// Enable thinking
	// Requirements:
	//   - Temperature must be set to 1
	thinkingBudget := anthropicThinkingBudgetTokens
	if config.ThinkingBudgetTokens != nil {
		thinkingBudget = *config.ThinkingBudgetTokens
	}
	claudeConfig.Thinking = &claudemodel.Thinking{
		Enable:       config.ThinkingEnabled, // from config
		BudgetTokens: thinkingBudget,
	}

	// Wire response schema into anthropic's sdk-go's output_config.format via eino's AdditionalRequestFields.
	// The eino Claude wrapper doesn't expose output_config natively, but it passes
	// AdditionalRequestFields through option.WithJSONSet (sjson dot-path format).
	if config.ResponseSchema != nil {
		schemaBytes, err := json.Marshal(config.ResponseSchema)
		if err != nil {
			return nil, NewInvalidConfigError(Anthropic, "failed to marshal response schema: "+err.Error())
		}
		var schemaMap map[string]any
		if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
			return nil, NewInvalidConfigError(Anthropic, "failed to convert response schema: "+err.Error())
		}
		claudeConfig.AdditionalRequestFields = map[string]any{
			"output_config.format": map[string]any{
				"type":   "json_schema",
				"schema": schemaMap,
			},
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

		if strings.Contains(strings.ToLower(err.Error()), "prompt is too long") {
			return "", NewTokenLimitError(Anthropic, m.GetId(), err.Error())
		}

		if strings.Contains(strings.ToLower(err.Error()), "too many requests") {
			return "", NewRateLimitError(Anthropic, m.GetId(), err.Error())
		}

		err = errors.Wrap(err, "failed to generate llm response")
		return "", NewModelUnavailableError(Anthropic, m.GetId(), err.Error())
	}

	return flattenResponseMessage(response), nil
}
