package storage

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

func (tree *bPlusTree) SearchTree(key int) (any, bool) {
	curr := tree.rootNode

	for {
		if curr.IsLeaf {
			for i, k := range curr.Keys {
				if k == key {
					return curr.Values[i], true
				}
			}
			return nil, false
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
