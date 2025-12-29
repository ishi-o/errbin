package errbin

// errorNode represents a node in an error tree,
// containing error information, error handler, and child nodes.
type errorNode struct {
	Error    error
	Handler  ErrorHandler
	Parent   *errorNode
	Children []*errorNode
}

var errorTree = make([]*errorNode, 0)

func removeRoots(nodes []int) {
	for _, idx := range nodes {
		errorTree = append(errorTree[:idx], errorTree[idx+1:]...)
	}
}
