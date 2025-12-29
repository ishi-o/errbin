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
// containing error information, error handler, and child nodes.
type ErrorNode struct {
	error    error
	handler  ErrorHandler
	parent   *ErrorNode
	children []*ErrorNode
}

var errorTree []*ErrorNode

// Register associates error handlers with errors.
//
// NOTE: This function is NOT concurrent-safe and must be called
// during application initialization only.
func Register(handler ErrorHandler, errs ...error) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}
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
		if chidx, children := findChildren(newErr); len(children) > 0 {
			newNode := &ErrorNode{
				error:    newErr,
				handler:  handler,
				children: children,
			}
			removeRoots(chidx)
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
	var trave func(nodes []*ErrorNode) (*ErrorNode, *ErrorNode)
	trave = func(nodes []*ErrorNode) (*ErrorNode, *ErrorNode) {
		for _, node := range nodes {
			if errors.Is(target, node.error) {
				if errors.Is(node.error, target) {
					return node.parent, node
				}
				if parent, child := trave(node.children); child != nil {
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

func findChildren(newErr error) (chidx []int, children []*ErrorNode) {
	for i := len(errorTree) - 1; i >= 0; i-- {
		root := errorTree[i]
		if errors.Is(root.error, newErr) {
			chidx = append(chidx, i)
			children = append(children, root)
		}
	}
	return
}

func removeRoots(nodes []int) {
	for _, idx := range nodes {
		errorTree = append(errorTree[:idx], errorTree[idx+1:]...)
	}
}

func findHandler(err error) (ErrorHandler, bool) {
	parent, itself := findPosition(err)
	if itself != nil {
		return itself.handler, true
	} else if parent != nil {
		return parent.handler, true
	} else {
		return nil, false
	}
}

var fallbackHandler ErrorHandler = func(err error, ctx *gin.Context) {
	ctx.JSON(http.StatusInternalServerError, gin.H{
		"error": "Unhandled error",
	})
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
			fallbackHandler(err, c)
		}
	}
}

// Fallback allows to set a customize default/fallback ErrorHandler
func Fallback(fn ErrorHandler) {
	fallbackHandler = fn
}
