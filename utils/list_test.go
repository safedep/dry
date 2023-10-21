package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindInSlice(t *testing.T) {
	cases := []struct {
		name  string
		items []string
		item  string
		idx   int
	}{
		{
			"When string is present in slice",
			[]string{"Hello", "World"},
			"World",
			1,
		},
		{
			"When string is not present in slice",
			[]string{"Hello", "World"},
			"None",
			-1,
		},
		{
			"When case is not matched",
			[]string{"Hello", "World"},
			"world",
			-1,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			idx := FindInSlice(test.items, test.item)
			assert.Equal(t, test.idx, idx)
		})
	}
}

func TestFindAnyWith(t *testing.T) {
	fn := func(item *map[string]string) bool {
		if _, ok := (*item)["a"]; ok {
			return true
		}

		return false
	}

	cases := []struct {
		name  string
		items []map[string]string
		found bool
	}{
		{
			"Item is found",
			[]map[string]string{
				{
					"a": "a",
				},
			},
			true,
		},
		{
			"Item is not found",
			[]map[string]string{
				{
					"b": "a",
				},
			},
			false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			x := FindAnyWith(test.items, fn)
			found := x != nil

			assert.Equal(t, found, test.found)
		})
	}
}
