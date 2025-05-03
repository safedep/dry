package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHuggingFaceHubClientImpl_AutoConfigureAPIToken(t *testing.T) {
	t.Run("should auto-configure the API token from the environment variable", func(t *testing.T) {
		os.Setenv("HF_TOKEN", "testtoken")
		defer os.Unsetenv("HF_TOKEN")

		client := NewHuggingFaceHubClient()
		assert.Equal(t, "testtoken", client.apiToken)
	})

	t.Run("should not configure the API token if it is explicitly set", func(t *testing.T) {
		os.Setenv("HF_TOKEN", "testtoken")
		defer os.Unsetenv("HF_TOKEN")

		client := NewHuggingFaceHubClient(WithAPIToken("explicit-token"))
		assert.Equal(t, "explicit-token", client.apiToken)
	})

	t.Run("should not configure the API token if it is not set", func(t *testing.T) {
		client := NewHuggingFaceHubClient()
		assert.Equal(t, "", client.apiToken)
	})
}

func TestHuggingFaceHubClientImpl_GetModel(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models/testowner/testmodel", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "Bearer testtoken", r.Header.Get("Authorization"))

		modelResp := map[string]interface{}{
			"_id":          "67ed3cd9290a7f9d3301f9c1",
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
			"library_name": "transformers",
			"sha":          "7dab2f5f854fe665b6b2f1eccbd3c48e5f627ad8",
			"disabled":     false,
			"gated":        "manual",
			"model-index":  nil,
			"license":      "mit",
			"safetensors":  true,
			"inference":    "warm",
			"usedStorage":  217343257722,
			"transformersInfo": map[string]interface{}{
				"auto_model":   "AutoModelForImageTextToText",
				"pipeline_tag": "image-text-to-text",
				"processor":    "AutoProcessor",
			},
			"siblings": []map[string]interface{}{
				{"rfilename": ".gitattributes"},
				{"rfilename": "LICENSE"},
				{"rfilename": "README.md"},
			},
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
		WithHTTPClient(mockServer.Client()),
	)

	ctx := context.Background()
	model, err := client.GetModel(ctx, "testowner", "testmodel")

	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "testowner/testmodel", model.ID)
	assert.Equal(t, "67ed3cd9290a7f9d3301f9c1", model.ModelID)
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
	assert.Equal(t, "transformers", model.LibraryName)
	assert.Equal(t, "mit", model.License)
	assert.Equal(t, "7dab2f5f854fe665b6b2f1eccbd3c48e5f627ad8", model.SHA)
	assert.False(t, model.Disabled)
	assert.Equal(t, "manual", model.Gated.(string))
	assert.Equal(t, "warm", model.Inference)
	assert.Equal(t, int64(217343257722), model.UsedStorage)
	assert.Equal(t, 3, len(model.SiblingModels))
	assert.Equal(t, ".gitattributes", model.SiblingModels[0].RFilename)
	assert.NotNil(t, model.TransformersInfo)
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
			"_id":          "67ed3cd9290a7f9d3301f9c2",
			"id":           "testowner/testdataset",
			"datasetId":    "testdataset",
			"author":       "testowner",
			"tags":         []string{"nlp", "text", "task_categories:question-answering", "license:cc-by-4.0"},
			"downloads":    500,
			"likes":        20,
			"createdAt":    "2023-01-01T00:00:00Z",
			"lastModified": "2023-02-01T00:00:00Z",
			"private":      false,
			"description":  "A test dataset for question answering",
			"citation":     "@article{test2023, title={Test Dataset}}",
			"license":      "cc-by-4.0",
			"sha":          "7dab2f5f854fe665b6b2f1eccbd3c48e5f627ad8",
			"disabled":     false,
			"gated":        "manual",
			"usedStorage":  3145728,
			"pretty_name":  "Test Dataset",
			"siblings": []map[string]interface{}{
				{"rfilename": "README.md"},
				{"rfilename": "dataset-info.json"},
				{"rfilename": "dataset_dict.json"},
			},
			"cardData": map[string]interface{}{
				"language":        []string{"en"},
				"license":         "cc-by-4.0",
				"task_categories": []string{"question-answering", "text-generation"},
				"pretty_name":     "Test Dataset",
				"size_categories": []string{"1M<n<10M"},
				"tags":            []string{"test", "qa"},
			},
			"configs": []map[string]interface{}{
				{
					"config_name": "default",
					"data_files": []map[string]interface{}{
						{
							"split": "train",
							"path":  "data/train-*",
						},
						{
							"split": "test",
							"path":  "data/test-*",
						},
					},
				},
			},
			"dataset_info": map[string]interface{}{
				"features": []map[string]interface{}{
					{
						"name":  "question",
						"dtype": "string",
					},
					{
						"name":  "answer",
						"dtype": "string",
					},
				},
				"splits": []map[string]interface{}{
					{
						"name":         "train",
						"num_bytes":    1000000,
						"num_examples": 10000,
					},
					{
						"name":         "test",
						"num_bytes":    200000,
						"num_examples": 2000,
					},
				},
				"download_size": 1200000,
				"dataset_size":  1200000,
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
	assert.Equal(t, "67ed3cd9290a7f9d3301f9c2", dataset.DatasetID)
	assert.Equal(t, "testdataset", dataset.DatasetName)
	assert.Equal(t, "testowner", dataset.Author)
	assert.Equal(t, 4, len(dataset.Tags))
	assert.Contains(t, dataset.Tags, "nlp")
	assert.Contains(t, dataset.Tags, "text")
	assert.Contains(t, dataset.Tags, "task_categories:question-answering")
	assert.Equal(t, int64(500), dataset.Downloads)
	assert.Equal(t, 20, dataset.Likes)
	assert.Equal(t, "2023-01-01T00:00:00Z", dataset.CreatedAt)
	assert.Equal(t, "2023-02-01T00:00:00Z", dataset.LastModified)
	assert.False(t, dataset.Private)
	assert.Equal(t, "A test dataset for question answering", dataset.Description)
	assert.Equal(t, "@article{test2023, title={Test Dataset}}", dataset.Citation)
	assert.Equal(t, "cc-by-4.0", dataset.License)
	assert.Equal(t, "7dab2f5f854fe665b6b2f1eccbd3c48e5f627ad8", dataset.SHA)
	assert.False(t, dataset.Disabled)
	assert.Equal(t, "manual", dataset.Gated.(string))
	assert.Equal(t, int64(3145728), dataset.UsedStorage)
	assert.Equal(t, "Test Dataset", dataset.PrettyName)

	// Test new fields
	assert.Equal(t, 3, len(dataset.SiblingDatasets))
	assert.Equal(t, "README.md", dataset.SiblingDatasets[0].RFilename)
	assert.NotNil(t, dataset.CardData)

	// Check configs
	assert.Equal(t, 1, len(dataset.Configs))
	assert.Equal(t, "default", dataset.Configs[0].ConfigName)
	assert.Equal(t, 2, len(dataset.Configs[0].DataFiles))
	assert.Equal(t, "train", dataset.Configs[0].DataFiles[0].Split)
	assert.Equal(t, "data/train-*", dataset.Configs[0].DataFiles[0].Path)

	// Check dataset info
	assert.NotNil(t, dataset.DatasetInfo)
	assert.Equal(t, 2, len(dataset.DatasetInfo.Features))
	assert.Equal(t, "question", dataset.DatasetInfo.Features[0].Name)
	assert.Equal(t, "string", dataset.DatasetInfo.Features[0].Dtype)
	assert.Equal(t, 2, len(dataset.DatasetInfo.Splits))
	assert.Equal(t, "train", dataset.DatasetInfo.Splits[0].Name)
	assert.Equal(t, int64(10000), dataset.DatasetInfo.Splits[0].NumExamples)
	assert.Equal(t, int64(1200000), dataset.DatasetInfo.DownloadSize)
	assert.Equal(t, int64(1200000), dataset.DatasetInfo.DatasetSize)

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

func TestHuggingFaceHubClient_GetModel_E2E(t *testing.T) {
	cases := []struct {
		name   string
		owner  string
		model  string
		assert func(t *testing.T, model *HuggingFaceModel, err error)
	}{
		{
			name:  "Using a valid public model",
			owner: "meta-llama",
			model: "Llama-4-Scout-17B-16E-Instruct",
			assert: func(t *testing.T, model *HuggingFaceModel, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, model)
				assert.Contains(t, model.ID, "meta-llama/Llama-4-Scout-17B-16E-Instruct")
				assert.Equal(t, "meta-llama", model.Author)

				// Verify fields from the example JSON structure
				assert.NotEmpty(t, model.ModelID)
				assert.True(t, len(model.Tags) > 0)
				assert.NotEmpty(t, model.SHA)
				assert.NotNil(t, model.SiblingModels)
				assert.Greater(t, model.Downloads, int64(0))
				assert.Greater(t, model.Likes, 0)
				assert.NotEmpty(t, model.CreatedAt)
				assert.NotEmpty(t, model.LastModified)

				// Validate complex fields
				if model.TransformersInfo != nil {
					assert.NotEmpty(t, model.TransformersInfo["pipeline_tag"])
				}

				// Verify SafeTensor details
				assert.NotNil(t, model.SafeTensors)
			},
		},
		{
			name:  "Non-existent model",
			owner: "nonexistent",
			model: "nonexistent",
			assert: func(t *testing.T, model *HuggingFaceModel, err error) {
				assert.Nil(t, model)
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHuggingFaceHubClient()
			model, err := client.GetModel(context.Background(), tc.owner, tc.model)
			tc.assert(t, model, err)
		})
	}
}

func TestHuggingFaceHubClient_GetDataset_E2E(t *testing.T) {
	cases := []struct {
		name    string
		owner   string
		dataset string
		assert  func(t *testing.T, dataset *HuggingFaceDataset, err error)
	}{
		{
			name:    "Using a valid public dataset",
			owner:   "nvidia",
			dataset: "OpenMathReasoning",
			assert: func(t *testing.T, dataset *HuggingFaceDataset, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, dataset)
				assert.Contains(t, dataset.ID, "nvidia/OpenMathReasoning")
				assert.Equal(t, "nvidia", dataset.Author)

				// Check Gated field which can be either a string or boolean
				assert.NotNil(t, dataset.Gated)

				// Check for other expected fields
				assert.NotEmpty(t, dataset.SHA)
				assert.Greater(t, dataset.Downloads, int64(0))
				assert.Greater(t, dataset.Likes, 0)
				assert.NotEmpty(t, dataset.CreatedAt)
				assert.NotEmpty(t, dataset.LastModified)
				assert.NotEmpty(t, dataset.Tags)
			},
		},
		{
			name:    "Non-existent dataset",
			owner:   "nonexistent",
			dataset: "nonexistent",
			assert: func(t *testing.T, dataset *HuggingFaceDataset, err error) {
				assert.Nil(t, dataset)
				assert.Error(t, err)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHuggingFaceHubClient()
			dataset, err := client.GetDataset(context.Background(), tc.owner, tc.dataset)
			tc.assert(t, dataset, err)
		})
	}
}
