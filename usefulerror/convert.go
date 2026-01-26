package usefulerror

import (
	"fmt"
	"sync"

	"github.com/safedep/dry/log"
)

const (
	registryIdentifierInternal    = "internal"
	registryIdentifierApplication = "application"
)

// ErrorConverterFunc is the contract for implementing error convertors across different error types
type ErrorConverterFunc func(err error) (UsefulError, bool)

// registryIdentifierMap is a map of registry identifiers used for deduplication of registry identifiers.
var registryIdentifierMap = make(map[string]bool)

// internalErrorConverterRegistry is a registry of error converters for different error types.
var internalErrorConverterRegistry = make([]ErrorConverterFunc, 0)

// applicationErrorConverterRegistry is a registry of error converters for application-specific error types.
var applicationErrorConverterRegistry = make([]ErrorConverterFunc, 0)

// errorConverterRegistryMutex is a mutex for synchronizing access to error converter registries.
var errorConverterRegistryMutex sync.RWMutex

// convertToUsefulError converts an error into a UsefulError. This enumerates all
// registered converters and returns the first one that can convert the error.
// Application registered error converters take precedence over internal error converters.
func convertToUsefulError(err error) (UsefulError, bool) {
	if err == nil {
		return nil, false
	}

	errorConverterRegistryMutex.RLock()
	defer errorConverterRegistryMutex.RUnlock()

	for _, converterFunc := range applicationErrorConverterRegistry {
		usefulErr, ok := converterFunc(err)
		if ok {
			return usefulErr, true
		}
	}

	for _, converterFunc := range internalErrorConverterRegistry {
		usefulErr, ok := converterFunc(err)
		if ok {
			return usefulErr, true
		}
	}

	return nil, false
}

// RegisterErrorConverter registers a new error converter for a given identifier.
// This is the public API for registering error converters by application code.
// Attempting to register a duplicate identifier will panic. This is done to prevent
// silent overwriting of existing error converters.
func RegisterErrorConverter(identifier string, converterFunc ErrorConverterFunc) {
	registerErrorConverter(registryIdentifierApplication, identifier, converterFunc)
}

// registerInternalErrorConverters registers internal error converters for standard error types.
func registerInternalErrorConverters(identifier string, converterFunc ErrorConverterFunc) {
	registerErrorConverter(registryIdentifierInternal, identifier, converterFunc)
}

// registerErrorConverter registers a new error converter for a given identifier in a given registry.
func registerErrorConverter(registryType string, identifier string, converterFunc ErrorConverterFunc) {
	errorConverterRegistryMutex.Lock()
	defer errorConverterRegistryMutex.Unlock()

	prefixedIdentifier := fmt.Sprintf("%s/%s", registryType, identifier)

	if _, exists := registryIdentifierMap[prefixedIdentifier]; exists {
		panic(fmt.Sprintf("error converter with identifier %s already registered", prefixedIdentifier))
	}

	log.Debugf("Registering error converter for identifier: %s", prefixedIdentifier)

	registryIdentifierMap[prefixedIdentifier] = true

	if registryType == registryIdentifierInternal {
		internalErrorConverterRegistry = append(internalErrorConverterRegistry, converterFunc)
	} else {
		applicationErrorConverterRegistry = append(applicationErrorConverterRegistry, converterFunc)
	}
}
