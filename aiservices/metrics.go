package aiservices

import "github.com/safedep/dry/obs"

var (
	metricAiServicesLlmGenerationTotal = obs.NewCounterVec(
		"aisvc_llm_generation_total",
		"Total number of LLM generation requests",
		[]string{"provider", "model"},
	)

	metricAiServicesLlmGenerationErrors = obs.NewCounterVec(
		"aisvc_llm_generation_errors_total",
		"Total number of LLM generation errors",
		[]string{"provider", "model", "error_type"},
	)
)
