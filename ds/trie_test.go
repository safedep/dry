package ds

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	words = []string{"apple", "banana", "cat", "dog", "elephant", "frog",
		"grape", "hello", "ice cream", "java", "kangaroo",
		"lemon", "monkey", "nectarine", "orange", "python",
		"quail", "rabbit", "strawberry", "turtle", "unicorn",
		"vegetable", "watermelon", "xylophone", "yak", "zebra",
		"preference", "prefix", "president", "precaution", "premium",
		"rebuild", "recharge", "reconnect", "rearrange", "reconsider",
		"leet", "leetcode",
	}
)

func setup() *Trie {
	trie := New()

	for _, word := range words {
		trie.Insert(word)
	}

	return trie
}

func TestContains(t *testing.T) {
	trie := setup()
	assertion := assert.New(t)

	assertion.True(trie.Contains("apple"), "Expected `true`")
	assertion.True(trie.Contains("strawberry"), "Expected `true`")
	assertion.True(trie.Contains("nectarine"), "Expected `true`")

	assertion.False(trie.Contains("mango"), "Expected `false`")
	assertion.False(trie.Contains("berry"), "Expected `false`")
	assertion.False(trie.Contains("tomato"), "Expected `false`")
}

func TestContainsPrefix(t *testing.T) {
	trie := setup()
	assertion := assert.New(t)

	assertion.True(trie.ContainsPrefix("re"), "Expected `true`")
	assertion.True(trie.ContainsPrefix("rec"), "Expected `true`")
	assertion.True(trie.ContainsPrefix("reb"), "Expected `true`")
	assertion.True(trie.ContainsPrefix("rearrange"), "Expected `true`")
	assertion.True(trie.ContainsPrefix("pre"), "Expected `true`")
	assertion.True(trie.ContainsPrefix("a"), "Expected `true`")

	assertion.False(trie.ContainsPrefix("zzzz"), "Expected `false`")
	assertion.False(trie.ContainsPrefix("boo"), "Expected `false`")
	assertion.False(trie.ContainsPrefix("foo"), "Expected `false`")
	assertion.False(trie.ContainsPrefix("got"), "Expected `false`")
}

func TestWordsWithPrefix(t *testing.T) {

	type testCase struct {
		input    string
		expected []string
	}

	testCases := []testCase{
		{
			input:    "app",
			expected: []string{"apple"},
		},
		{
			input:    "re",
			expected: []string{"rebuild", "recharge", "reconnect", "rearrange", "reconsider"},
		},
		{
			input:    "rec",
			expected: []string{"recharge", "reconnect", "reconsider"},
		},
		{
			input:    "pre",
			expected: []string{"preference", "prefix", "president", "precaution", "premium"},
		},
		{
			input:    "leet",
			expected: []string{"leet", "leetcode"},
		},
		{
			input:    "foo",
			expected: []string{},
		},
		{
			input:    "bat",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("Input::%s", tc.input)

		t.Run(name, func(t *testing.T) {
			trie := setup()

			assertion := assert.New(t)

			actual := trie.WordsWithPrefix(tc.input)

			assertion.ElementsMatchf(tc.expected, actual, "Expected %v but got %v\n", tc.expected, actual)
		})

	}

}

func TestDelete(t *testing.T) {
	type testCase struct {
		input                      string
		inputForContains           string
		expectedForContains        bool
		inputForContainsPrefix     string
		expectedForContainsPrefix  bool
		inputForWordsWithPrefix    string
		expectedForWordsWithPrefix []string
	}

	testCases := []testCase{
		{
			input:                      "apple",
			inputForContains:           "apple",
			expectedForContains:        false,
			inputForContainsPrefix:     "apple",
			expectedForContainsPrefix:  false,
			inputForWordsWithPrefix:    "apple",
			expectedForWordsWithPrefix: []string{},
		},
		{
			input:                      "reb",
			inputForContains:           "rebuild",
			expectedForContains:        true,
			inputForContainsPrefix:     "reb",
			expectedForContainsPrefix:  true,
			inputForWordsWithPrefix:    "reb",
			expectedForWordsWithPrefix: []string{"rebuild"},
		},
		{
			input:                      "re",
			inputForContains:           "rebuild",
			expectedForContains:        true,
			inputForContainsPrefix:     "re",
			expectedForContainsPrefix:  true,
			inputForWordsWithPrefix:    "re",
			expectedForWordsWithPrefix: []string{"rebuild", "recharge", "reconnect", "rearrange", "reconsider"},
		},
		{
			input:                      "rebuild",
			inputForContains:           "rebuild",
			expectedForContains:        false,
			inputForContainsPrefix:     "reb",
			expectedForContainsPrefix:  false,
			inputForWordsWithPrefix:    "re",
			expectedForWordsWithPrefix: []string{"recharge", "reconnect", "rearrange", "reconsider"},
		},
		{
			input:                      "leetcode",
			inputForContains:           "leet",
			expectedForContains:        true,
			inputForContainsPrefix:     "leetc",
			expectedForContainsPrefix:  false,
			inputForWordsWithPrefix:    "leetc",
			expectedForWordsWithPrefix: []string{},
		},
		{
			input:                      "foo",
			inputForContains:           "leet",
			expectedForContains:        true,
			inputForContainsPrefix:     "leet",
			expectedForContainsPrefix:  true,
			inputForWordsWithPrefix:    "leet",
			expectedForWordsWithPrefix: []string{"leet", "leetcode"},
		},
	}

	var (
		actual   interface{}
		expected interface{}
		msgFmt   = "Expected %v but got back %v\n"
	)

	for _, tc := range testCases {
		name := fmt.Sprintf("Input::%s", tc.input)
		t.Run(name, func(t *testing.T) {
			trie := setup()

			assertion := assert.New(t)

			trie.Delete(tc.input)

			actual = trie.Contains(tc.inputForContains)
			expected = tc.expectedForContains
			assertion.Equalf(expected, actual, msgFmt, expected, actual)

			actual = trie.ContainsPrefix(tc.inputForContainsPrefix)
			expected = tc.expectedForContainsPrefix
			assertion.Equalf(expected, actual, msgFmt, expected, actual)

			actual = trie.WordsWithPrefix(tc.inputForWordsWithPrefix)
			expected = tc.expectedForWordsWithPrefix
			assertion.ElementsMatchf(expected, actual, msgFmt, expected, actual)
		})
	}

}
