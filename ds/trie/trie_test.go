package trie

import (
	"fmt"
	"testing"

	"github.com/safedep/dry/utils"
	"github.com/stretchr/testify/assert"
)

var (
	// We're using same words as string values for simplicity
	setupWords = []string{"apple", "banana", "grape", "grapevine", "persimmons", "persistent"}
)

func setup() *Trie[string] {
	trie := NewTrie[string]()

	for _, word := range setupWords {
		trie.Insert(word, utils.PtrTo(word))
	}

	return trie
}

func TestGetWord(t *testing.T) {
	trie := setup()
	assertion := assert.New(t)

	// Test for words that exist in the trie (added during setup)
	for _, word := range setupWords {
		value, exists := trie.GetWord(word)
		assertion.True(exists)
		assertion.Equal(word, *value)
	}

	// Test for words that do not exist in the trie
	for _, word := range []string{"", "mango", "persim"} {
		value, exists := trie.GetWord(word)
		assertion.False(exists)
		assertion.Nil(value)
	}
}

func TestContainsPrefix(t *testing.T) {
	trie := setup()
	assertion := assert.New(t)

	assertion.True(trie.ContainsPrefix(""))
	assertion.True(trie.ContainsPrefix("app"))
	assertion.True(trie.ContainsPrefix("gr"))
	assertion.True(trie.ContainsPrefix("grape"))
	assertion.True(trie.ContainsPrefix("grapevine"))

	assertion.False(trie.ContainsPrefix("z"))
	assertion.False(trie.ContainsPrefix("boo"))
}

func TestWordsWithPrefix(t *testing.T) {
	type testCase struct {
		input         string
		expectedWords []string
	}

	testCases := []testCase{
		{
			input:         "",
			expectedWords: setupWords,
		},
		{
			input:         "app",
			expectedWords: []string{"apple"},
		},
		{
			input:         "gr",
			expectedWords: []string{"grape", "grapevine"},
		},
		{
			input:         "grape",
			expectedWords: []string{"grape", "grapevine"},
		},
		{
			input:         "grapevine",
			expectedWords: []string{"grapevine"},
		},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("Input-%s", tc.input)
		trie := setup()

		t.Run(name, func(t *testing.T) {
			assertion := assert.New(t)
			expected := make([]TrieWordEntry[string], len(tc.expectedWords))
			for i, word := range tc.expectedWords {
				expected[i].Word = word
				expected[i].Value = utils.PtrTo(word)
			}
			assertion.ElementsMatch(expected, trie.WordsWithPrefix(tc.input))
		})
	}
}

func TestDelete(t *testing.T) {
	type testCase struct {
		deleteKey                             string
		postDeleteGetWordKey                  string
		expectedPostDeleteGetWordKeyExistence bool
		postDeleteContainsPrefixKey           string
		expectedPostDeleteContainsPrefix      bool
		postDeleteWordsWithPrefixKey          string
		expectedPostDeleteWordsWithPrefix     []string
	}

	testCases := []testCase{
		{
			deleteKey:                             "apple",
			postDeleteGetWordKey:                  "apple",
			expectedPostDeleteGetWordKeyExistence: false,
			postDeleteContainsPrefixKey:           "apple",
			expectedPostDeleteContainsPrefix:      false,
			postDeleteWordsWithPrefixKey:          "apple",
			expectedPostDeleteWordsWithPrefix:     []string{},
		},
		{
			deleteKey:                             "g",
			postDeleteGetWordKey:                  "grape",
			expectedPostDeleteGetWordKeyExistence: true,
			postDeleteContainsPrefixKey:           "g",
			expectedPostDeleteContainsPrefix:      true,
			postDeleteWordsWithPrefixKey:          "g",
			expectedPostDeleteWordsWithPrefix:     []string{"grape", "grapevine"},
		},
		{
			deleteKey:                             "grape",
			postDeleteGetWordKey:                  "grapevine",
			expectedPostDeleteGetWordKeyExistence: true,
			postDeleteContainsPrefixKey:           "grape",
			expectedPostDeleteContainsPrefix:      true,
			postDeleteWordsWithPrefixKey:          "grape",
			expectedPostDeleteWordsWithPrefix:     []string{"grapevine"},
		},
		{
			deleteKey:                             "grapevine",
			postDeleteGetWordKey:                  "grapevine",
			expectedPostDeleteGetWordKeyExistence: false,
			postDeleteContainsPrefixKey:           "grapevine",
			expectedPostDeleteContainsPrefix:      false,
			postDeleteWordsWithPrefixKey:          "grape",
			expectedPostDeleteWordsWithPrefix:     []string{"grape"},
		},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("Input-%s", tc.deleteKey)
		t.Run(name, func(t *testing.T) {
			trie := setup()

			assertion := assert.New(t)

			trie.Delete(tc.deleteKey)

			// Check if the word is deleted
			value, exists := trie.GetWord(tc.deleteKey)
			assertion.False(exists)
			assertion.Nil(value)

			value, exists = trie.GetWord(tc.postDeleteGetWordKey)
			assertion.Equal(tc.expectedPostDeleteGetWordKeyExistence, exists)
			if tc.expectedPostDeleteGetWordKeyExistence {
				assertion.Equal(tc.postDeleteGetWordKey, *value)
			} else {
				assertion.Nil(value)
			}

			assertion.Equal(tc.expectedPostDeleteContainsPrefix, trie.ContainsPrefix(tc.postDeleteContainsPrefixKey))

			actual := trie.WordsWithPrefix(tc.postDeleteWordsWithPrefixKey)
			expected := make([]TrieWordEntry[string], len(tc.expectedPostDeleteWordsWithPrefix))
			for i, word := range tc.expectedPostDeleteWordsWithPrefix {
				expected[i].Word = word
				expected[i].Value = utils.PtrTo(word)
			}
			assertion.ElementsMatch(expected, actual)
		})
	}
}
