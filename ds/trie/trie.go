// Inspiration: https://github.com/priyakdey/trie

package trie

// node is a single node in the prefix tree, which represents a letter in word
type node[T any] struct {
	// key is the int representation of the character in a word
	key uint8

	// children is a map of node
	children map[uint8]*node[T]

	// isWord is a marker to determine if the node marks a word in the tree
	isWord bool

	// value is a pointer to data associated with this word (nil if no data or not a word)
	value *T
}

// Trie represents the Prefix Tree.
// This should not be created directly, instead use trie.NewTrie()
type Trie[T any] struct {
	root *node[T]
}

func NewTrie[T any]() *Trie[T] {
	return &Trie[T]{
		root: &node[T]{
			key:      0,
			children: make(map[uint8]*node[T]),
			isWord:   false,
			value:    nil,
		},
	}
}

// inserts the given word into the trie with an associated value
// If a word exists already, it is updated with new provided value
func (t *Trie[T]) Insert(word string, value *T) {
	// Can't insert empty strings
	if word == "" {
		return
	}

	children := t.root.children

	var n *node[T]
	var ok bool

	for i := 0; i < len(word); i++ {
		ch := word[i]
		n, ok = children[ch]
		if !ok {
			n = &node[T]{
				key:      ch,
				children: make(map[uint8]*node[T]),
				isWord:   false,
				value:    nil,
			}
			children[ch] = n
		}

		children = n.children
	}

	n.isWord = true
	n.value = value
}

// If the word is present in trie, it returns the value associated with the word and true - indicating it exists.
// If the word is not present, it returns (nil, false) - indicating it does not exist.
// Empty string (""), will always return (nil, false) since "" is not a word in any dictionary.
func (t *Trie[T]) GetWord(word string) (value *T, exists bool) {
	node, ok := t.search(word)

	if !ok || !node.isWord {
		return nil, false
	}

	return node.value, true
}

// Checks whether given prefix is present in the tree.
// To get all words which start with this prefix, use WordsWithPrefix.
func (t *Trie[T]) ContainsPrefix(prefix string) bool {
	_, ok := t.search(prefix)

	return ok
}

type TrieWordEntry[T any] struct {
	Word  string
	Value *T
}

// WordsWithPrefix returns a list of all entries which start with the prefix
func (t *Trie[T]) WordsWithPrefix(prefix string) []TrieWordEntry[T] {
	resultEntries := make([]TrieWordEntry[T], 0)

	node, ok := t.search(prefix)

	if !ok {
		return resultEntries
	}

	t.addWords(node, prefix, &resultEntries)

	return resultEntries
}

// Delete removes the given word from the trie
func (t *Trie[T]) Delete(word string) {
	if word == "" {
		return
	}

	children := t.root.children

	var (
		// visitedNodes is the list of all nodes visited while reaching the `word``
		visitedNodes = make([]*node[T], 0)
		node         *node[T]
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
	node.value = nil

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

func (t *Trie[T]) search(prefix string) (*node[T], bool) {
	children := t.root.children

	var (
		node *node[T] = t.root
		ok   bool
	)

	for i := 0; i < len(prefix); i++ {
		ch := prefix[i]
		node, ok = children[ch]
		if !ok {
			return nil, false
		}

		children = node.children
	}

	return node, true
}

func (t *Trie[T]) addWords(node *node[T], currentPrefix string, words *[]TrieWordEntry[T]) {
	if node.isWord {
		*words = append(*words, TrieWordEntry[T]{Word: currentPrefix, Value: node.value})
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
		t.addWords(_node, currentPrefix+string(key), words)
	}
}
