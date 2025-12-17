package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	sig_yaml "sigs.k8s.io/yaml"

	"github.com/go-viper/mapstructure/v2"
)

// Serialize an interface using JSON or return error string
func Introspect(v interface{}) string {
	bytes, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error())
	} else {
		return string(bytes)
	}
}

func CleanPath(path string) string {
	return filepath.Clean(path)
}

func MapStruct[T any](source interface{}, dest *T) error {
	return mapstructure.Decode(source, dest)
}

func SafelyGetValue[T any](target *T) T {
	var vi T
	if target != nil {
		vi = *target
	}

	return vi
}

func IsEmptyString(s string) bool {
	return s == ""
}

// Deprecated: Use `api/pb` package instead
func ToPbJson[T proto.Message](obj T, indent string) (string, error) {
	m := jsonpb.Marshaler{Indent: indent, OrigName: true}
	return m.MarshalToString(obj)
}

// Deprecated: Use `api/pb` package instead
func FromPbJson[T proto.Message](reader io.Reader, obj T) error {
	return jsonpb.Unmarshal(reader, obj)
}

// Deprecated: Use `api/pb` package instead
func FromYamlToPb[T proto.Message](reader io.Reader, obj T) error {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return err
	}

	jsonData, err := sig_yaml.YAMLToJSON(buf.Bytes())
	if err != nil {
		return err
	}

	return jsonpb.Unmarshal(bytes.NewReader(jsonData), obj)
}

// Deprecated: Use `api/pb` package instead
func FromPbToYaml[T proto.Message](writer io.Writer, obj T) error {
	jsonData, err := ToPbJson(obj, "")
	if err != nil {
		return err
	}

	yamlData, err := sig_yaml.JSONToYAML([]byte(jsonData))
	if err != nil {
		return err
	}

	_, err = writer.Write(yamlData)
	if err != nil {
		return err
	}

	return nil
}

// TrimWithEllipsis trims the string `s` to `maxLength` characters.
// If `centered` is true, it shows the start and end of the string with ellipsis in the middle.
// The ellipsis length is controlled by `dots`.
// If remaining characters after dots are odd, extra character is shown on the prefix side.
func TrimWithEllipsis(s string, maxLength int, centered bool, dots int) string {
	if maxLength <= 0 || dots < 0 {
		return ""
	}

	if len(s) <= maxLength {
		return s
	}

	if dots == 0 || maxLength <= dots {
		return s[:maxLength]
	}

	ellipsis := strings.Repeat(".", dots)

	if !centered {
		trimLen := maxLength - dots
		if trimLen <= 0 {
			return s[:maxLength]
		}
		return s[:trimLen] + ellipsis
	}

	remaining := maxLength - dots
	if remaining <= 0 {
		return s[:maxLength]
	}

	// Extra character to prefix if odd
	leftLen := (remaining + 1) / 2
	rightLen := remaining - leftLen

	return s[:leftLen] + ellipsis + s[len(s)-rightLen:]
}

// StringStripQuotes removes surrounding quote characters from a string
// From string it removes outer most quote pair
// - "abc" -> abc
// - 'abc' -> abc
// - \'abc\' -> abc
// - `abc` -> abc
func StringStripQuotes(s string) string {
	if len(s) < 2 {
		return s
	}

	if (s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '\'' && s[len(s)-1] == '\'') ||
		(s[0] == '`' && s[len(s)-1] == '`') {
		return s[1 : len(s)-1]
	}

	return s
}
