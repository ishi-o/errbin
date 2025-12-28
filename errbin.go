// Package errbin provides declarative error handling for Gin.
package errbin

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorHandler is a function type that handles errors in Gin HTTP requests.
// It receives an error and a Gin context pointer
// to manage the HTTP request/response lifecycle.
type ErrorHandler func(error, *gin.Context)

// ErrorNode 表示错误树中的一个节点，包含错误信息、错误处理器和子节点
// ErrorNode represents a node in an error tree, containing error information, error handler, and child nodes
type ErrorNode struct {
	error    error
	handler  ErrorHandler
	children []*ErrorNode
}

var errorTree []*ErrorNode

func Register(handler ErrorHandler, errs ...error) error {
	for _, newErr := range errs {
		if newErr == nil {
			return fmt.Errorf("cannot register nil error")
		}
		// if node already exists
		if node := findExactNode(newErr); node != nil {
			return fmt.Errorf("duplicate registration: %v", newErr)
		}
		// if node is a child of another node
		if parent := findParentFor(newErr); parent != nil {
			parent.children = append(parent.children, &ErrorNode{
				error:   newErr,
				handler: handler,
			})
			continue
		}
		// if node is a father of another one
		if children := findChildrenFor(newErr); len(children) > 0 {
			newNode := &ErrorNode{
				error:    newErr,
				handler:  handler,
				children: children,
			}
			removeNodesFromRoots(children)
			errorTree = append(errorTree, newNode)
			continue
		}
		// otherwise as a new node
		errorTree = append(errorTree, &ErrorNode{
			error:   newErr,
			handler: handler,
		})
	}
	return nil
}

func findExactNode(target error) *ErrorNode {
	var dfs func(nodes []*ErrorNode) *ErrorNode
	dfs = func(nodes []*ErrorNode) *ErrorNode {
		for _, node := range nodes {
			if errors.Is(target, node.error) {
				if errors.Is(node.error, target) {
					return node
				}
				if child := dfs(node.children); child != nil {
					return child
				}
				// if Is(target, node.error) is true, target mustbe
				// node.error itself, or its sons
				return nil
			}
		}
		return nil
	}
	return dfs(errorTree)
}

func findParentFor(newErr error) *ErrorNode {
	for _, root := range errorTree {
		if parent := dfsFindParent(root, newErr); parent != nil {
			return parent
		}
	}
	return nil
}

func dfsFindParent(node *ErrorNode, newErr error) *ErrorNode {
	if errors.Is(newErr, node.error) {
		for _, child := range node.children {
			if errors.Is(newErr, child.error) {
				if deeper := dfsFindParent(child, newErr); deeper != nil {
					return deeper
				}
			}
		}
		return node
	}
	return nil
}

func findChildrenFor(newErr error) []*ErrorNode {
	var children []*ErrorNode
	for i := 0; i < len(errorTree); i++ {
		root := errorTree[i]
		if errors.Is(root.error, newErr) {
			children = append(children, root)
		}
	}

	return children
}

func removeNodesFromRoots(nodes []*ErrorNode) {
	for _, node := range nodes {
		for i := 0; i < len(errorTree); i++ {
			if errorTree[i] == node {
				errorTree = append(errorTree[:i], errorTree[i+1:]...)
				i--
				break
			}
		}
	}
}

func findHandler(err error) (ErrorHandler, bool) {
	var queue []*ErrorNode
	queue = append(queue, errorTree...)

	var bestMatch *ErrorNode

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if errors.Is(err, node.error) {
			bestMatch = node
			queue = append(queue, node.children...)
		}
	}

	if bestMatch != nil {
		return bestMatch.handler, true
	}
	return nil, false
}

// ErrbinMiddleware return a gin.HandleFunc as a middleware
func ErrbinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		if handler, found := findHandler(err); found {
			handler(err, c)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Unhandled error",
			})
		}
	}
}
