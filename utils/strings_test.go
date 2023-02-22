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
