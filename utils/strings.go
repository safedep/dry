package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

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
