// Source: https://github.com/priyakdey/trie

package ds

import (
	"strings"
)

// node is a single node in the prefix tree, which represents the a letter in word
type node struct {
	// key is the int representation of the character in a word
	key uint8

	// children is a map of node
	children map[uint8]*node

	// isWord is a marker to determine if the node marks a word in the tree
	isWord bool
}

// Trie represents the Prefix Tree.
// This should not be created directly, instead use trie.New()
type Trie struct {
	root *node
}

func New() *Trie {
	return &Trie{
		root: &node{
			key:      0,
			children: make(map[uint8]*node),
			isWord:   false,
		},
	}
}

func (t *Trie) Insert(word string) {
	children := t.root.children

	var n *node
	var ok bool

	for i := 0; i < len(word); i++ {
		ch := word[i]
		n, ok = children[ch]
		if !ok {
			n = &node{
				key:      ch,
				children: make(map[uint8]*node),
				isWord:   false,
			}
			children[ch] = n
		}

		children = n.children
	}

	n.isWord = true

}

// Checks whether the given word is present in the tree.
// Empty string (""), will always return false since "" is
// not a word in any dictionary.
func (t *Trie) Contains(word string) bool {
	node, ok := t.search(word)

	if !ok {
		return false
	}

	return node.isWord
}

// Checks whether any words are present in the tree starting with
// the given prefix similar to strings.HasPrefix()
// To get all words which star twith this prefix, use WordsWithPrefix.
func (t *Trie) ContainsPrefix(prefix string) bool {
	_, ok := t.search(prefix)

	return ok
}

// WordsWithPrefix returns a list of all words which starts with the prefix.
func (t *Trie) WordsWithPrefix(prefix string) []string {
	words := make([]string, 0)

	node, ok := t.search(prefix)

	if !ok {
		return words
	}

	// Since node is present, means prefix is present in the Trie.
	// Prefill the buf with all characters from prefix so we can append
	// to it to generate words.
	buf := make([]string, len(prefix))

	for i, ch := range prefix {
		buf[i] = string(ch)
	}

	t.addWords(node, &buf, &words)

	return words
}

// Delete removes the given word from the dictionary.
func (t *Trie) Delete(word string) {
	children := t.root.children

	var (
		// visitedNodes is the list of all nodes visited while reaching the `word``
		visitedNodes = make([]*node, 0)
		node         *node
		ok           bool
	)

	for i := 0; i < len(word); i++ {
		ch := word[i]
		node, ok = children[ch]
		if !ok {
			return
		}

		visitedNodes = append(visitedNodes, node)
		children = node.children
	}

	// set the isWord marker to false - soft delete the word if present
	node.isWord = false

	// iterate from the last node of the list and drop the node if no branches
	// from that prefix or node is not a word marker
	// no children = no branches form that node
	for i := len(visitedNodes) - 1; i >= 1; i-- {
		n := visitedNodes[i]

		if len(n.children) == 0 && !n.isWord {
			parent := visitedNodes[i-1]
			delete(parent.children, n.key) // delete the reference from the parent node
		}
	}

	// check for the first character
	firstCh := word[0]
	n := t.root.children[firstCh]
	if len(n.children) == 0 && !n.isWord {
		delete(t.root.children, n.key)
	}

}

func (t *Trie) search(word string) (*node, bool) {
	children := t.root.children

	var (
		node *node
		ok   bool
	)

	for i := 0; i < len(word); i++ {
		ch := word[i]
		node, ok = children[ch]
		if !ok {
			return nil, false
		}

		children = node.children
	}

	return node, true
}

func (t *Trie) addWords(node *node, buf *[]string, words *[]string) {
	if node.isWord {
		word := strings.Join(*buf, "")
		*words = append(*words, word)
	}

	children := node.children

	// make sure to init children whenever creating node, to avoid null ptr
	if len(children) == 0 {
		// exit recursion for leaf nodes
		return
	}

	// iterate over children of each children of node
	// traverse all possible branches from node.children
	for key, _node := range children {
		*buf = append(*buf, string(key))
		t.addWords(_node, buf, words)
		*buf = (*buf)[:len(*buf)-1]
	}
}
