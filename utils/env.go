package utils

import (
	"os"
	"strconv"
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
