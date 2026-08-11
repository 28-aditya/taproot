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

func (tree *bPlusTree) findLeaf(key int) (*Node) {

	curr:= tree.rootNode

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
		for i,k := range leaf.Keys {
			if k == key {
				return leaf.Values[i], true
			}
		}
	} 

	return nil, false
}
