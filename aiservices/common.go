package aiservices

import (
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// flattenResponseMessage takes an eino's []*schema.Message and concatenates its Content and MultiContent
// fields into a single string. This is useful for extracting the full text of a response
func flattenResponseMessage(response *schema.Message) string {
	result := response.Content

	for _, message := range response.MultiContent {
		result += message.Text
	}

	return result
}

func modelInferenceOptionsToEinoModelOptions(opts []inferenceOptionFn) []model.Option {
	modelInferenceOptions := new(ModelInferenceOptions)
	for _, opt := range opts {
		opt(modelInferenceOptions)
	}

	// generateOptions are eino preventive used when calling GenerateSingle
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
