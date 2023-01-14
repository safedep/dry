package errors

// Optional error specific attributes
type ApiError_Params struct {
	AdditionalProperties map[string]struct {
		Key   *string `json:"key,omitempty"`
		Value *string `json:"value,omitempty"`
	} `json:"-"`
}

// ApiError defines model for ApiError.
type ApiError struct {
	// An error code identifying the error
	Code *string `json:"code,omitempty"`

	// A descriptive message about the error meant for developer consumption
	Message *string `json:"message,omitempty"`

	// Optional error specific attributes
	Params *ApiError_Params `json:"params,omitempty"`

	// An optional service or domain specific error group
	Type *string `json:"type,omitempty"`
}
