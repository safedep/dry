package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHuggingFaceHubClientImpl_GetModel(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models/testowner/testmodel", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "Bearer testtoken", r.Header.Get("Authorization"))

		modelResp := map[string]interface{}{
			"id":           "testowner/testmodel",
			"modelId":      "testmodel",
			"author":       "testowner",
			"tags":         []string{"nlp", "transformers"},
			"downloads":    1000,
			"likes":        42,
			"createdAt":    "2023-01-01T00:00:00Z",
			"lastModified": "2023-02-01T00:00:00Z",
			"private":      false,
			"pipeline_tag": "text-classification",
			"library":      "transformers",
			"license":      "mit",
			"safetensors":  true,
			"cardData": map[string]interface{}{
				"language": "en",
				"license":  "mit",
			},
			"metrics": []map[string]interface{}{
				{
					"type":  "accuracy",
					"value": 0.95,
					"name":  "Accuracy",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(modelResp)
	}))
	defer mockServer.Close()

	client := NewHuggingFaceHubClient(
		WithBaseURL(mockServer.URL),
		WithAPIToken("testtoken"),
		WithTimeout(5*time.Second),
	)

	ctx := context.Background()
	model, err := client.GetModel(ctx, "testowner", "testmodel")

	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "testowner/testmodel", model.ID)
	assert.Equal(t, "testmodel", model.ModelName)
	assert.Equal(t, "testowner", model.Author)
	assert.Equal(t, 2, len(model.Tags))
	assert.Contains(t, model.Tags, "nlp")
	assert.Contains(t, model.Tags, "transformers")
	assert.Equal(t, int64(1000), model.Downloads)
	assert.Equal(t, 42, model.Likes)
	assert.Equal(t, "2023-01-01T00:00:00Z", model.CreatedAt)
	assert.Equal(t, "2023-02-01T00:00:00Z", model.LastModified)
	assert.False(t, model.Private)
	assert.Equal(t, "text-classification", model.PipelineTag)
	assert.Equal(t, "transformers", model.Library)
	assert.Equal(t, "mit", model.License)
	assert.True(t, model.SafeTensors)
	assert.NotNil(t, model.CardData)
	assert.Equal(t, 1, len(model.Metrics))
	assert.Equal(t, "accuracy", model.Metrics[0].Type)
	assert.Equal(t, 0.95, model.Metrics[0].Value)
	assert.Equal(t, "Accuracy", model.Metrics[0].Name)
	assert.NotNil(t, model.RawResponse)
}

func TestHuggingFaceHubClientImpl_GetDataset(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/datasets/testowner/testdataset", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		datasetResp := map[string]interface{}{
			"id":           "testowner/testdataset",
			"datasetId":    "testdataset",
			"author":       "testowner",
			"tags":         []string{"nlp", "text"},
			"downloads":    500,
			"likes":        20,
			"createdAt":    "2023-01-01T00:00:00Z",
			"lastModified": "2023-02-01T00:00:00Z",
			"private":      false,
			"description":  "A test dataset",
			"citation":     "@article{test2023, title={Test Dataset}}",
			"license":      "cc-by-4.0",
			"size":         1048576,
			"cardData": map[string]interface{}{
				"language": "en",
				"license":  "cc-by-4.0",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(datasetResp)
	}))
	defer mockServer.Close()

	client := NewHuggingFaceHubClient(
		WithBaseURL(mockServer.URL),
	)

	ctx := context.Background()
	dataset, err := client.GetDataset(ctx, "testowner", "testdataset")

	assert.NoError(t, err)
	assert.NotNil(t, dataset)
	assert.Equal(t, "testowner/testdataset", dataset.ID)
	assert.Equal(t, "testdataset", dataset.DatasetName)
	assert.Equal(t, "testowner", dataset.Author)
	assert.Equal(t, 2, len(dataset.Tags))
	assert.Contains(t, dataset.Tags, "nlp")
	assert.Contains(t, dataset.Tags, "text")
	assert.Equal(t, int64(500), dataset.Downloads)
	assert.Equal(t, 20, dataset.Likes)
	assert.Equal(t, "2023-01-01T00:00:00Z", dataset.CreatedAt)
	assert.Equal(t, "2023-02-01T00:00:00Z", dataset.LastModified)
	assert.False(t, dataset.Private)
	assert.Equal(t, "A test dataset", dataset.Description)
	assert.Equal(t, "@article{test2023, title={Test Dataset}}", dataset.Citation)
	assert.Equal(t, "cc-by-4.0", dataset.License)
	assert.Equal(t, int64(1048576), dataset.Size)
	assert.NotNil(t, dataset.CardData)
	assert.NotNil(t, dataset.RawResponse)
}

func TestHuggingFaceHubClient_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Model not found"}`))
	}))
	defer mockServer.Close()

	client := NewHuggingFaceHubClient(
		WithBaseURL(mockServer.URL),
	)

	ctx := context.Background()
	_, err := client.GetModel(ctx, "nonexistent", "model")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestHuggingFaceHubClient_NetworkError(t *testing.T) {
	// Use a closed server to simulate network error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mockServer.Close()

	client := NewHuggingFaceHubClient(
		WithBaseURL(mockServer.URL),
		WithTimeout(100*time.Millisecond),
	)

	ctx := context.Background()
	_, err := client.GetModel(ctx, "owner", "model")

	assert.Error(t, err)
}

func TestHuggingFaceHubClient_InvalidResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"invalid`)) // Malformed JSON
	}))
	defer mockServer.Close()

	client := NewHuggingFaceHubClient(
		WithBaseURL(mockServer.URL),
	)

	ctx := context.Background()
	_, err := client.GetModel(ctx, "owner", "model")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse model response")
}

func TestHuggingFaceHubClient_E2E(t *testing.T) {
	cases := []struct {
		name   string
		owner  string
		model  string
		assert func(t *testing.T, model *HuggingFaceModel)
	}{
		{
			name:  "Using a valid public model",
			owner: "meta-llama",
			model: "Llama-4-Scout-17B-16E-Instruct",
			assert: func(t *testing.T, model *HuggingFaceModel) {
				assert.NotNil(t, model)
				assert.Equal(t, "meta-llama/Llama-4-Scout-17B-16E-Instruct", model.ID)
				assert.Equal(t, "Llama-4-Scout-17B-16E-Instruct", model.ModelName)
				assert.Equal(t, "meta-llama", model.Author)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHuggingFaceHubClient()
			model, err := client.GetModel(context.Background(), tc.owner, tc.model)
			assert.NoError(t, err)
			tc.assert(t, model)
		})
	}
}
