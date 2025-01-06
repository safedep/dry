package pb

import (
	"bytes"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	sig_yaml "sigs.k8s.io/yaml"
)

// Serialize a protocol buffers message to JSON. This function explicitly
// uses the proto field names for JSON keys.
func ToJson[T proto.Message](obj T, indent string) ([]byte, error) {
	m := protojson.MarshalOptions{Indent: indent, UseProtoNames: true}
	return m.Marshal(obj)
}

// Deserialize a protocol buffers message from JSON.
func FromJson[T proto.Message](reader io.Reader, obj T) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	return protojson.Unmarshal(data, obj)
}

// Deserialize a protocol buffers message from YAML. Internally
// it converts the YAML to JSON and then unmarshals the JSON.
func FromYaml[T proto.Message](reader io.Reader, obj T) error {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return err
	}

	jsonData, err := sig_yaml.YAMLToJSON(buf.Bytes())
	if err != nil {
		return err
	}

	return FromJson(bytes.NewReader(jsonData), obj)
}

// Serialize a protocol buffers message to YAML. Internally it
// converts the message to JSON and then converts the JSON to YAML.
func ToYaml[T proto.Message](writer io.Writer, obj T) error {
	jsonData, err := ToJson(obj, "")
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
