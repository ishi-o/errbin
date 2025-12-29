package errbin

import "errors"

func findPosition(target error) (*errorNode, *errorNode) {
	var trave func(nodes []*errorNode) (*errorNode, *errorNode)
	trave = func(nodes []*errorNode) (*errorNode, *errorNode) {
		for _, node := range nodes {
			if errors.Is(target, node.Error) {
				if errors.Is(node.Error, target) {
					return node.Parent, node
				} else if parent, child := trave(node.Children); child != nil {
					return parent, child
				} else if parent == nil {
					// if errors.Is(target, node.error) is true, target mustbe
					// node.error itself, or its sons
					return node, nil
				} else {
					return parent, nil
				}
			}
		}
		return nil, nil
	}
	return trave(errorTree)
}

func findChildren(newErr error) (chidx []int, children []*errorNode) {
	for i := len(errorTree) - 1; i >= 0; i-- {
		root := errorTree[i]
		if errors.Is(root.Error, newErr) {
			chidx = append(chidx, i)
			children = append(children, root)
		}
	}
	return
}

func findHandler(err error) (ErrorHandler, bool) {
	parent, itself := findPosition(err)
	if itself != nil {
		return itself.Handler, true
	} else if parent != nil {
		return parent.Handler, true
	} else {
		return nil, false
	}
}
