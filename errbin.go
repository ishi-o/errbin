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

// ErrorNode represents a node in an error tree,
// containing error information, error handler, and child nodes
type ErrorNode struct {
	error    error
	handler  ErrorHandler
	parent   *ErrorNode
	children []*ErrorNode
}

var errorTree []*ErrorNode

func Register(handler ErrorHandler, errs ...error) error {
	for _, newErr := range errs {
		if newErr == nil {
			return fmt.Errorf("cannot register nil error")
		}
		parent, itself := findPosition(newErr)
		if itself != nil { // if node already exists
			return fmt.Errorf("duplicate registration: %v", newErr)
		} else if parent != nil { // if node is a child of another node
			parent.children = append(parent.children, &ErrorNode{
				error:   newErr,
				handler: handler,
				parent:  parent,
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

func findPosition(target error) (*ErrorNode, *ErrorNode) {
	var dfs func(nodes []*ErrorNode) (*ErrorNode, *ErrorNode)
	dfs = func(nodes []*ErrorNode) (*ErrorNode, *ErrorNode) {
		for _, node := range nodes {
			if errors.Is(target, node.error) {
				if errors.Is(node.error, target) {
					return node.parent, node
				}
				if parent, child := dfs(node.children); child != nil {
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
	return dfs(errorTree)
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
