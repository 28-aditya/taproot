package storage

const KEYCAPACITY = 4

type bPlusTree struct {
	rootNode *Node
}

type Tree = bPlusTree

func (tree *bPlusTree) setRoot(root *Node) {
	tree.rootNode = root
}

func NewTree() *bPlusTree {
	initialRoot := &Node{IsLeaf: true, dirty: true}
	newTree := bPlusTree{rootNode: initialRoot}
	return &newTree
}

func (tree *bPlusTree) findLeaf(key int) (*Node, []*Node) {
	curr := tree.rootNode
	var path []*Node

	for {
		if curr.IsLeaf {
			return curr, path
		}

		path = append(path, curr)

		childIndex := len(curr.Pointers) - 1
		for i, k := range curr.Keys {
			if key < k {
				childIndex = i
				break
			}
		}

		curr = curr.Pointers[childIndex]
	}
}

func (tree *bPlusTree) SearchTree(key int) (any, bool) {
	leaf, _ := tree.findLeaf(key)

	for i, k := range leaf.Keys {
		if k == key {
			return leaf.Values[i], true
		}
	}

	return nil, false
}

func (tree *bPlusTree) Insert(key int, value any) error {
	leaf, path := tree.findLeaf(key)

	i := sortedIntIndex(leaf.Keys, key)
	leaf.Keys = insertInt(leaf.Keys, i, key)
	leaf.Values = insertAny(leaf.Values, i, value)
	leaf.dirty = true

	if len(leaf.Keys) > KEYCAPACITY {
		newLeaf, promotedKey := tree.splitLeaf(leaf)
		tree.insertIntoParent(path, leaf, promotedKey, newLeaf)
	}

	return nil
}

func (tree *bPlusTree) splitLeaf(leaf *Node) (*Node, int) {
	mid := len(leaf.Keys) / 2

	newLeaf := &Node{
		IsLeaf: true,
		Keys:   append([]int{}, leaf.Keys[mid:]...),
		Values: append([]any{}, leaf.Values[mid:]...),
		dirty:  true,
	}

	leaf.Keys = leaf.Keys[:mid]
	leaf.Values = leaf.Values[:mid]

	newLeaf.Next = leaf.Next
	leaf.Next = newLeaf
	leaf.dirty = true

	return newLeaf, newLeaf.Keys[0]
}

func (tree *bPlusTree) splitInternal(node *Node) (*Node, int) {
	mid := len(node.Keys) / 2
	promotedKey := node.Keys[mid]

	newNode := &Node{
		IsLeaf:   false,
		Keys:     append([]int{}, node.Keys[mid+1:]...),
		Pointers: append([]*Node{}, node.Pointers[mid+1:]...),
		dirty:    true,
	}

	node.Keys = node.Keys[:mid]
	node.Pointers = node.Pointers[:mid+1]
	node.dirty = true

	return newNode, promotedKey
}

func (tree *bPlusTree) insertIntoParent(path []*Node, left *Node, key int, right *Node) {
	if len(path) == 0 {
		newRoot := &Node{
			IsLeaf:   false,
			Keys:     []int{key},
			Pointers: []*Node{left, right},
			dirty:    true,
		}
		tree.setRoot(newRoot)
		return
	}

	parent := path[len(path)-1]

	idx := 0
	for i, p := range parent.Pointers {
		if p == left {
			idx = i
			break
		}
	}

	parent.Keys = insertInt(parent.Keys, idx, key)
	parent.Pointers = insertNode(parent.Pointers, idx+1, right)
	parent.dirty = true

	if len(parent.Keys) > KEYCAPACITY {
		newParent, promotedKey := tree.splitInternal(parent)
		tree.insertIntoParent(path[:len(path)-1], parent, promotedKey, newParent)
	}
}

func (tree bPlusTree) RangeScan(lowerLimit int, upperLimit int) map[int]any {
	scan := map[int]any{}
	leaf, _ := tree.findLeaf(lowerLimit)

	for leaf != nil {
		done := false
		for i, k := range leaf.Keys {
			if k > upperLimit {
				done = true
				break
			}
			if k >= lowerLimit {
				scan[k] = leaf.Values[i]
			}
		}
		if done {
			break
		}
		leaf = leaf.Next
	}

	return scan
}

func (tree bPlusTree) DeleteLeaf(key int) bool {
	leaf, _ := tree.findLeaf(key)

	for i, k := range leaf.Keys {
		if k == key {
			leaf.Keys = append(leaf.Keys[:i], leaf.Keys[i+1:]...)
			leaf.Values = append(leaf.Values[:i], leaf.Values[i+1:]...)
			leaf.dirty = true
			return true
		}
	}

	return false
}

// helper functions

func sortedIntIndex(keys []int, key int) int {
	i := len(keys)
	for j, k := range keys {
		if k > key {
			i = j
			break
		}
	}
	return i
}

func insertInt(s []int, i int, v int) []int {
	return append(s[:i], append([]int{v}, s[i:]...)...)
}

func insertAny(s []any, i int, v any) []any {
	return append(s[:i], append([]any{v}, s[i:]...)...)
}

func insertNode(s []*Node, i int, v *Node) []*Node {
	return append(s[:i], append([]*Node{v}, s[i:]...)...)
}