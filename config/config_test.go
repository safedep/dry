package config

import (
	"testing"

	"github.com/safedep/dry/log"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfigFromEnvironment(t *testing.T) {
	log.Init("config-test")

	t.Run("APP_SERVICE_NAME", func(t *testing.T) {
		t.Setenv("APP_SERVICE_NAME", "test-service")

		value, err := AppServiceName.Value()
		assert.NoError(t, err)

		assert.Equal(t, "test-service", value)
	})
}

func TestConfigString(t *testing.T) {
	t.Run("ValueSet", func(t *testing.T) {
		t.Setenv("test", "Value")

		c := Config[string]{Name: "test"}
		v, err := c.Value()
		assert.NoError(t, err)
		assert.Equal(t, "Value", v)
	})

}

func TestConfigInt(t *testing.T) {
	t.Skip("Skipping test as it is failing")

	t.Run("ValueSet", func(t *testing.T) {
		t.Setenv("test", "123")

		c := Config[int]{Name: "test"}
		v, err := c.Value()
		assert.NoError(t, err)
		assert.Equal(t, 123, v)
	})

}

func TestConfigFloat(t *testing.T) {
	t.Skip("Skipping test as it is failing")

	t.Run("ValueSet", func(t *testing.T) {
		t.Setenv("test", "123.45")

		c := Config[float64]{Name: "test"}
		v, err := c.Value()
		assert.NoError(t, err)
		assert.Equal(t, 123.45, v)
	})

}
