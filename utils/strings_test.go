package utils

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/stretchr/testify/assert"
)

func TestFromYamlToPb(t *testing.T) {
	cases := []struct {
		name    string
		yamlStr string
		pbMsg   *envoy_core_v3.Address
		errMsg  string
	}{
		{
			"Deserialize correctly",
			`socket_address:
  address: "1.2.3.4"
  port_value: 1234`,
			&envoy_core_v3.Address{
				Address: &envoy_core_v3.Address_SocketAddress{
					SocketAddress: &envoy_core_v3.SocketAddress{
						Address:       "1.2.3.4",
						PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{PortValue: 1234},
					},
				},
			},
			"",
		},
		{
			"Deserialize bad YAML",
			`AAAAAAA`,
			&envoy_core_v3.Address{},
			"json: cannot unmarshal string into Go value of type map[string]json.RawMessage",
		},
		{
			"Deserialize good YAML but incompatible with Proto Msg",
			`A: 1`,
			&envoy_core_v3.Address{},
			"unknown field \"A\" in envoy.config.core.v3.Address",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var obj envoy_core_v3.Address
			err := FromYamlToPb(strings.NewReader(test.yamlStr), &obj)

			if test.errMsg != "" {
				assert.EqualError(t, err, test.errMsg)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.pbMsg.GetSocketAddress().GetAddress(), obj.GetSocketAddress().GetAddress())
				assert.Equal(t, test.pbMsg.GetSocketAddress().GetPortValue(), obj.GetSocketAddress().GetPortValue())
			}
		})
	}
}

func TestFromPbToYAML(t *testing.T) {
	cases := []struct {
		name         string
		pbMsg        *envoy_core_v3.Address
		yamlContains string
		errMsg       string
	}{
		{
			"YAML contains address",
			&envoy_core_v3.Address{
				Address: &envoy_core_v3.Address_SocketAddress{
					SocketAddress: &envoy_core_v3.SocketAddress{
						Address:       "1.2.3.4",
						PortSpecifier: &envoy_core_v3.SocketAddress_PortValue{PortValue: 1234},
					},
				},
			},
			"address: 1.2.3.4",
			"",
		},
		{
			"YAML does not contains address",
			&envoy_core_v3.Address{
				Address: &envoy_core_v3.Address_SocketAddress{},
			},
			"socket_address: {}",
			"",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			buffer := bytes.Buffer{}
			writer := bufio.NewWriter(&buffer)

			err := FromPbToYaml(writer, test.pbMsg)
			if test.errMsg != "" {
				assert.NotNil(t, err)
				assert.ErrorContains(t, err, test.errMsg)
			} else {
				writer.Flush()

				assert.Nil(t, err)
				assert.Contains(t, string(buffer.Bytes()), test.yamlContains)
			}
		})
	}
}

func TestTrimWithEllipsis(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		maxLength int
		centered  bool
		dots      int
		expected  string
	}{
		{
			name:      "No trimming required",
			s:         "Hello",
			maxLength: 10,
			centered:  false,
			dots:      3,
			expected:  "Hello",
		},
		{
			name:      "No trimming required (centered)",
			s:         "Hello",
			maxLength: 10,
			centered:  true,
			dots:      3,
			expected:  "Hello",
		},
		{
			name:      "Exact length, no trimming required",
			s:         "HelloThere",
			maxLength: 10,
			centered:  false,
			dots:      3,
			expected:  "HelloThere",
		},
		{
			name:      "Exact length, no trimming required (centered)",
			s:         "HelloThere",
			maxLength: 10,
			centered:  true,
			dots:      3,
			expected:  "HelloThere",
		},
		{
			name:      "Prefix trimming",
			s:         "HelloWorldExample",
			maxLength: 10,
			centered:  false,
			dots:      3,
			expected:  "HelloWo...",
		},
		{
			name:      "Centered trimming with equal prefix and suffix",
			s:         "Hello Everyone",
			maxLength: 10,
			centered:  true,
			dots:      2,
			expected:  "Hell..yone",
		},
		{
			name:      "Centered trimming unequal prefix & suffix (must show extra prefix)",
			s:         "Hello everyone",
			maxLength: 10,
			centered:  true,
			dots:      3,
			expected:  "Hell...one",
		},
		{
			name:      "Zero max length",
			s:         "Hello",
			maxLength: 0,
			centered:  false,
			dots:      3,
			expected:  "",
		},
		{
			name:      "Zero max length (centered)",
			s:         "Hello",
			maxLength: 0,
			centered:  true,
			dots:      3,
			expected:  "",
		},
		{
			name:      "Dots zero",
			s:         "HelloWorldExample",
			maxLength: 10,
			centered:  false,
			dots:      0,
			expected:  "HelloWorld",
		},
		{
			name:      "Dots zero (centered)",
			s:         "HelloWorldExample",
			maxLength: 10,
			centered:  true,
			dots:      0,
			expected:  "HelloWorld",
		},
		{
			name:      "Max length and dots zero",
			s:         "HelloWorldExample",
			maxLength: 0,
			centered:  true,
			dots:      0,
			expected:  "",
		},
		{
			name:      "Max length and dots zero (centered)",
			s:         "HelloWorldExample",
			maxLength: 0,
			centered:  true,
			dots:      0,
			expected:  "",
		},
		{
			name:      "Dots larger than string",
			s:         "Hi",
			maxLength: 5,
			centered:  false,
			dots:      5,
			expected:  "Hi",
		},
		{
			name:      "Dots larger than string (centered)",
			s:         "Hi",
			maxLength: 5,
			centered:  true,
			dots:      5,
			expected:  "Hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimWithEllipsis(tt.s, tt.maxLength, tt.centered, tt.dots)
			assert.Equal(t, tt.expected, result, "TrimWithEllipsis failed for case: %s", tt.name)
		})
	}
}

func TestStringStripQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard Cases
		{"Double Quotes", `"hello"`, "hello"},
		{"Single Quotes", `'world'`, "world"},
		{"Backticks", "`golang`", "golang"},

		// No Quotes / No Change
		{"No Quotes", "plain text", "plain text"},
		{"Leading Quote Only", `"missing end`, `"missing end`},
		{"Trailing Quote Only", `missing start"`, `missing start"`},
		{"Mismatched Quotes", ` 'mismatch"`, ` 'mismatch"`},

		// Edge Cases: Length
		{"Empty String", "", ""},
		{"Single Quote Char", `"`, `"`},
		{"Two Matching Quotes Only", `""`, ""},
		{"Two Mismatched Quotes", `"'`, `"'`},

		// Internal Quotes (should remain)
		{"Internal Double Quotes", `"he"llo"`, `he"llo`},
		{"Quotes inside different quotes", `"'quoted'"`, `'quoted'`},

		// Spaces
		{"Spaces outside quotes", ` "space" `, ` "space" `},
		{"Spaces inside quotes", `" space "`, ` space `},

		// Escaped Quotes
		{"Escaped Quote Inside", `"word\"word"`, `word\"word`},
		{"Backslash at Start", `\path\`, `\path\`},
		{"Escaped Newline", `"\n"`, `\n`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringStripQuotes(tt.input)
			if result != tt.expected {
				t.Errorf("StringStripQuotes(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}
