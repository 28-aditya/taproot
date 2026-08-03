package storage

type bPlusTree struct {
	rootNode *Node
}

func (currTree *bPlusTree) setRoot(root *Node) {
	currTree.rootNode = root
}

func NewTree() *bPlusTree {
	initialRoot := &Node{IsLeaf: true}

	newTree := bPlusTree{
		rootNode: initialRoot,
	}

	return &newTree
}
