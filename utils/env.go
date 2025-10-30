package utils

import (
	"os"
	"strconv"

	"github.com/caarlos0/env/v11"
)

// EnvBool looks up environment variable by name
// and converts the string value to bool. It returns
// default value if env does not exist or conversion
// to bool fails
func EnvBool(name string, def bool) bool {
	val, ok := os.LookupEnv(name)
	if !ok {
		return def
	}

	bRet, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}

	return bRet
}

// ParseEnvToStruct parses environment variables into a struct using the caarlos0/env package.
// The struct type T should have appropriate `env` and `envDefault` tags to define the mapping.
//
// Example usage:
//
//	type Config struct {
//	    Port int `env:"PORT" envDefault:"8080"`
//	    Host string `env:"HOST,required"`
//	}
//
//	config, err := ParseEnvToStruct[Config]()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// See https://github.com/caarlos0/env for full documentation on supported tags and features.
func ParseEnvToStruct[T any]() (T, error) {
	var config T
	if err := env.Parse(&config); err != nil {
		return config, err
	}
	return config, nil
}

// ParseEnvToStructWithOptions parses environment variables into a struct with custom options.
// This allows setting a prefix, using custom parsers, etc.
//
// Example usage:
//
//	config, err := ParseEnvToStructWithOptions[Config](env.Options{
//	    Prefix: "APP_",
//	})
func ParseEnvToStructWithOptions[T any](opts env.Options) (T, error) {
	var config T
	if err := env.ParseWithOptions(&config, opts); err != nil {
		return config, err
	}
	return config, nil
}
