package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// JSON based config type encoder
type JSONConfigEncoder[T any] struct{}

func (j *JSONConfigEncoder[T]) Encode(v T) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (j *JSONConfigEncoder[T]) Decode(s string) (T, error) {
	var v T

	err := json.Unmarshal([]byte(s), &v)
	if err != nil {
		return v, err
	}

	return v, nil
}

// Go strconv based config type encoder. It only supports
// decoding and do not support encoding
type strconvConfigEncoder[T any] struct{}

func (s *strconvConfigEncoder[T]) Decode(v string) (T, error) {
	var value T

	vt := reflect.TypeOf(value).Kind()
	switch vt {
	case reflect.String:
		value = reflect.ValueOf(v).Interface().(T)
	case reflect.Int, reflect.Int64:
		p, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			value = reflect.ValueOf(p).Interface().(T)
		} else {
			return value, err
		}
	case reflect.Float64:
		p, err := strconv.ParseFloat(v, 64)
		if err != nil {
			value = reflect.ValueOf(p).Interface().(T)
		} else {
			return value, err
		}
	default:
		return value, fmt.Errorf("strconvConfigEncoder does not support decoding %v", vt)
	}

	return value, nil
}

func (s *strconvConfigEncoder[T]) Encode(v T) (string, error) {
	return "", fmt.Errorf("strconvConfigEncoder does not support encoding")
}

// NewStrconvConfigEncoder returns a new instance of strconvConfigEncoder
// which is suitable only for decoding strings into Go types based on
// the type of the value obtained through reflection
func NewStrconvConfigEncoder[T any]() ConfigEncoder[T] {
	return &strconvConfigEncoder[T]{}
}
