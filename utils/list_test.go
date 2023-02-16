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
