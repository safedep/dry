package aiservices

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
)

func TestFlattenResponseMessage(t *testing.T) {
	responseMessage := &schema.Message{
		Content: "A",
		MultiContent: []schema.ChatMessagePart{
			{
				Text: "B",
			},
			{
				Text: "C",
			},
		},
	}

	result := flattenResponseMessage(responseMessage)
	assert.Equal(t, "ABC", result)
}

func Test_modelInferenceOptionsToEinoModelOptions(t *testing.T) {
	inferenceOptions := []inferenceOptionFn{
		WithTemperature(0.2),
		WithTopP(10),
		WithMaxTokens(100000),
		WithStopWords([]string{"stop", "word"}),
	}

	einoModelOptionList := modelInferenceOptionsToEinoModelOptions(inferenceOptions)
	assert.Equal(t, len(inferenceOptions), len(einoModelOptionList))
}
