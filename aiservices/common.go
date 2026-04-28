package aiservices

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// flattenResponseMessage concatenates a message's Content and multi-part fields into a single string.
func flattenResponseMessage(response *schema.Message) string {
	result := response.Content

	for _, part := range response.AssistantGenMultiContent {
		result += part.Text
	}

	return result
}

// modelInferenceOptionsToEinoModelOptions is a helper function to convert framework generic ModelInferenceOptions
// to eino's model.Option, which is then used in GenerateSingle like inference functions
func modelInferenceOptionsToEinoModelOptions(opts []inferenceOptionFn) []model.Option {
	modelInferenceOptions := new(ModelInferenceOptions)
	for _, opt := range opts {
		opt(modelInferenceOptions)
	}

	// generateOptions are eino primitive used when calling GenerateSingle
	var generateOptions []model.Option
	if modelInferenceOptions.Temperature != nil {
		generateOptions = append(generateOptions, model.WithTemperature(*modelInferenceOptions.Temperature))
	}
	if modelInferenceOptions.TopP != nil {
		generateOptions = append(generateOptions, model.WithTopP(*modelInferenceOptions.TopP))
	}
	if modelInferenceOptions.MaxTokens != nil {
		generateOptions = append(generateOptions, model.WithMaxTokens(*modelInferenceOptions.MaxTokens))
	}
	if len(modelInferenceOptions.StopWords) > 0 {
		generateOptions = append(generateOptions, model.WithStop(modelInferenceOptions.StopWords))
	}

	return generateOptions
}
