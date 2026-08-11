package storage

import ("errors")
const KEYCAPACITY = 4

type bPlusTree struct {
	rootNode *Node
}

func (tree *bPlusTree) setRoot(root *Node) {
	tree.rootNode = root
}

func NewTree() *bPlusTree {
	initialRoot := &Node{IsLeaf: true}

	newTree := bPlusTree{
		rootNode: initialRoot,
	}

	return &newTree
}

func (tree *bPlusTree) findLeaf(key int) *Node {

	curr := tree.rootNode

	for {
		if curr.IsLeaf {
			return curr
		}

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

	leaf := tree.findLeaf(key)

	if leaf != nil {
		for i, k := range leaf.Keys {
			if k == key {
				return leaf.Values[i], true
			}
		}
	}

	return nil, false
}

func (tree *bPlusTree) Insert(key int, value any) error {
	leaf := tree.findLeaf(key)

	if len(leaf.Keys) >= KEYCAPACITY {
		return errors.New("KEY CAPACITY EXCEEDED")
	}

	i := len(leaf.Keys)

	for j, k := range leaf.Keys {
		if k > key {
			i = j
			break
		}
	}
	
	leaf.Keys 	= 	append(leaf.Keys[:i], append([]int{key}, leaf.Keys[i:]...)...)
	leaf.Values = 	append(leaf.Values[:i], append([]any{value}, leaf.Values[i:]...)...)
	return nil
	
}
