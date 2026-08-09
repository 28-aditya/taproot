package storage

import ("fmt")
type Node struct {
	Keys   []int
	IsLeaf bool
	Pointers []*Node
	// vars for the leaf nodes (data stored and next node to the right)
	Values []any
	Next *Node
}

func nextLeaf(node Node) (*Node){
	if (node.IsLeaf){
		nextNode := node.Next
		if (nextNode != nil) {
			return nextNode
		}
		fmt.Printf("reached end of chain")
	}
	return nil
}

