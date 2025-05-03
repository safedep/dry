package huggingface

import "encoding/json"

// HuggingFaceModel represents metadata about a model in HuggingFace Hub
type HuggingFaceModel struct {
	ID               string            `json:"id"`               // Unique identifier of the model (owner/name)
	ModelID          string            `json:"_id,omitempty"`    // Internal ID in MongoDB format
	ModelName        string            `json:"modelId"`          // Name of the model
	Author           string            `json:"author"`           // Author of the model
	Tags             []string          `json:"tags"`             // Tags associated with the model
	Downloads        int64             `json:"downloads"`        // Number of downloads
	Likes            int               `json:"likes"`            // Number of likes
	CreatedAt        string            `json:"createdAt"`        // Creation date
	LastModified     string            `json:"lastModified"`     // Last modification date
	Private          bool              `json:"private"`          // Whether the model is private
	PipelineTag      string            `json:"pipeline_tag"`     // Pipeline tag
	LibraryName      string            `json:"library_name"`     // Library name for the model
	Library          string            `json:"library"`          // Associated library (legacy)
	CardData         map[string]any    `json:"cardData"`         // Model card data
	SiblingModels    []SiblingFile     `json:"siblings"`         // Sibling models - array of file objects
	ModelIndex       interface{}       `json:"model-index"`      // Model index information
	Config           map[string]any    `json:"config"`           // Model configuration
	SafeTensors      interface{}       `json:"safetensors"`      // Whether using safetensors or SafeTensor stats
	License          string            `json:"license"`          // License information
	Metrics          []MetricInfo      `json:"metrics"`          // Model metrics
	Disabled         bool              `json:"disabled"`         // Whether the model is disabled
	Gated            interface{}       `json:"gated"`            // Gating type - can be string or bool
	SHA              string            `json:"sha"`              // SHA hash of the model
	Spaces           []string          `json:"spaces"`           // Associated Spaces using this model
	TransformersInfo map[string]string `json:"transformersInfo"` // Transformers-specific information
	UsedStorage      int64             `json:"usedStorage"`      // Storage used by the model in bytes
	Inference        string            `json:"inference"`        // Inference type/status
	RawResponse      json.RawMessage   `json:"-"`                // Raw response from the API
}

// DatasetFeature represents a feature in a dataset
type DatasetFeature struct {
	Name  string `json:"name"`  // Name of the feature
	Dtype string `json:"dtype"` // Data type of the feature
}

// DatasetSplit represents a split in a dataset
type DatasetSplit struct {
	Name        string `json:"name"`         // Name of the split (e.g. "train", "test")
	NumBytes    int64  `json:"num_bytes"`    // Size of the split in bytes
	NumExamples int64  `json:"num_examples"` // Number of examples in the split
}

// DatasetConfig represents a configuration of a dataset
type DatasetConfig struct {
	ConfigName string            `json:"config_name"` // Name of the configuration
	DataFiles  []DatasetDataFile `json:"data_files"`  // Data files in the configuration
}

// DatasetDataFile represents a data file in a dataset configuration
type DatasetDataFile struct {
	Split string `json:"split"` // Split this file belongs to
	Path  string `json:"path"`  // Path to the file
}

// DatasetInfo represents information about a dataset
type DatasetInfo struct {
	Features     []DatasetFeature `json:"features"`      // Features in the dataset
	Splits       []DatasetSplit   `json:"splits"`        // Splits in the dataset
	DownloadSize int64            `json:"download_size"` // Size of the download in bytes
	DatasetSize  int64            `json:"dataset_size"`  // Total size of the dataset in bytes
}

// HuggingFaceDataset represents metadata about a dataset in HuggingFace Hub
type HuggingFaceDataset struct {
	ID              string         `json:"id"`                 // Unique identifier of the dataset (owner/name)
	DatasetID       string         `json:"_id,omitempty"`      // Internal ID in MongoDB format
	DatasetName     string         `json:"datasetId"`          // Name of the dataset
	Author          string         `json:"author"`             // Author of the dataset
	Tags            []string       `json:"tags"`               // Tags associated with the dataset
	Downloads       int64          `json:"downloads"`          // Number of downloads
	Likes           int            `json:"likes"`              // Number of likes
	CreatedAt       string         `json:"createdAt"`          // Creation date
	LastModified    string         `json:"lastModified"`       // Last modification date
	Private         bool           `json:"private"`            // Whether the dataset is private
	CardData        map[string]any `json:"cardData"`           // Dataset card data
	SiblingDatasets []SiblingFile  `json:"siblings"`           // Sibling dataset files
	Description     string         `json:"description"`        // Description of the dataset
	Citation        string         `json:"citation,omitempty"` // Citation information
	License         string         `json:"license,omitempty"`  // License information
	SHA             string         `json:"sha,omitempty"`      // SHA hash of the dataset
	Disabled        bool           `json:"disabled"`           // Whether the dataset is disabled
	Gated           interface{}    `json:"gated"`              // Gating type - can be string or bool
	UsedStorage     int64          `json:"usedStorage"`        // Storage used by the dataset in bytes

	// New fields from the example JSON
	PrettyName  string          `json:"pretty_name,omitempty"`  // Pretty name of the dataset
	Configs     []DatasetConfig `json:"configs,omitempty"`      // Dataset configurations
	DatasetInfo *DatasetInfo    `json:"dataset_info,omitempty"` // Dataset information

	RawResponse json.RawMessage `json:"-"` // Raw response from the API
}

// SiblingFile represents a file in the model repository
type SiblingFile struct {
	RFilename string `json:"rfilename"` // Relative filename
}

// MetricInfo represents metrics information for a model
type MetricInfo struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
	Name  string  `json:"name"`
}
