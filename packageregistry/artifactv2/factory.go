package artifactv2

import (
	"fmt"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// CreateAdapter creates an artifact adapter for the given ecosystem with optional configuration
func CreateAdapter(ecosystem packagev1.Ecosystem, opts ...Option) (ArtifactAdapterV2, error) {
	// Apply configuration
	config, err := applyOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	// Ensure defaults are set
	if err := config.ensureDefaults(); err != nil {
		return nil, fmt.Errorf("failed to initialize defaults: %w", err)
	}

	// Create ecosystem-specific adapter
	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_NPM:
		return &npmAdapterV2{
			config:  config,
			storage: config.storageManager,
		}, nil

	// Additional ecosystems will be added here as they are implemented
	// case packagev1.Ecosystem_ECOSYSTEM_PYPI:
	//     return newPyPiAdapterV2(config)
	// case packagev1.Ecosystem_ECOSYSTEM_GO:
	//     return newGoAdapterV2(config)
	// case packagev1.Ecosystem_ECOSYSTEM_CARGO:
	//     return newCargoAdapterV2(config)
	// case packagev1.Ecosystem_ECOSYSTEM_MAVEN:
	//     return newMavenAdapterV2(config)

	default:
		return nil, fmt.Errorf("unsupported ecosystem: %s", ecosystem.String())
	}
}
