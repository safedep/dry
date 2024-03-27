package config

// Config represents a generic configuration with metadata
// using which the configuration can be retrieved from a data source
// using a data source specific adapter. Example: Environment
type Config[T any] struct {
	Name    string
	Default T

	// The configuration value must be available in the data source
	// otherwise an error is returned.
	MustSupply bool

	// The repository to be used for retrieving the value
	// by default. If not provided, the environment repository
	// is used.
	Repository ConfigRepository[T]
}

// ConfigRepository defines the contract for implementing data source
// specific configuration adapters. This is used to retrieve the configuration
type ConfigRepository[T any] interface {
	GetConfig(*Config[T]) (T, error)
}

// ConfigEncoder defines the contract for implementing configuration
// encoders so that any complex type can be encoded into string for storage
// and decoded back to the original type.
type ConfigEncoder[T any] interface {
	Encode(T) (string, error)
	Decode(string) (T, error)
}

// Common configurations that spans across multiple services
// and are generally independ of domain specific configurations.
var (
	AppServiceName = Config[string]{Name: "APP_SERVICE_NAME"}
	AppServiceEnv  = Config[string]{Name: "APP_SERVICE_ENV"}
	AppLogFile     = Config[string]{Name: "APP_LOG_FILE"}
	AppLogLevel    = Config[string]{Name: "APP_LOG_LEVEL"}
)

// Helper function to access the value of a configuration
// using the default repository
func (c *Config[T]) Value() (T, error) {
	var err error
	r := c.Repository

	if r == nil {
		r, err = NewEnvironmentRepository(EnvironmentRepositoryConfig[T]{
			Encoder: NewStrconvConfigEncoder[T](),
		})
		if err != nil {
			return c.Default, err
		}
	}

	return r.GetConfig(c)
}

// Helper function to access the value of a configuration
// using a specific repository
func (c *Config[T]) ValueFrom(r ConfigRepository[T]) (T, error) {
	return r.GetConfig(c)
}
