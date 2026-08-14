package storage

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

	leaf, _:= tree.findLeaf(key)

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
	leaf, path := tree.findLeaf(key)

	if len(leaf.Keys) >= KEYCAPACITY {
		newLeaf, startKey := tree.SplitLeaf(leaf)

			if (len(path)==0) {
				newRoot:=Node{
					Keys:[]int{startKey},
					Pointers: []*Node{leaf,newLeaf},
					IsLeaf: false,
				}
				tree.setRoot(&newRoot)
			}

		if key>=startKey {
			leaf = newLeaf
			parentNode := path[len(path)-1]
			parentNode.Pointers = append(parentNode.Pointers, leaf)
		}
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


func (tree *bPlusTree) SplitLeaf(leaf *Node) (*Node, int) {
	mid := KEYCAPACITY/2
	newLeaf := &Node{
		IsLeaf: true,
		Keys: leaf.Keys[mid:],
		Values: leaf.Values[mid:],
	}
	leaf.Keys = leaf.Keys[:mid]
	leaf.Values = leaf.Values[:mid]
	
	newLeaf.Next = leaf.Next
	leaf.Next = newLeaf

	return newLeaf,newLeaf.Keys[0]
}