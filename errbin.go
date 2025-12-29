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

// ErrorMiddleware is a function type that execute between the last/next ErrorHandler.
type ErrorMiddleware func(ErrorHandler) ErrorHandler

// ErrorNode represents a node in an error tree,
// containing error information, error handler, and child nodes.
type ErrorNode struct {
	Error    error
	Handler  ErrorHandler
	Parent   *ErrorNode
	Children []*ErrorNode
}

var errorTree = make([]*ErrorNode, 0)

var globalMiddlewares ErrorMiddleware

var fallbackHandler ErrorHandler = func(err error, ctx *gin.Context) {
	ctx.JSON(http.StatusInternalServerError, gin.H{
		"error": err.Error(),
	})
}

// Use associates error handlers with errors.
//
// NOTE: This function is NOT concurrent-safe and must be called
// during application initialization only.
func Use(handler ErrorHandler, errs ...error) error {
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
			parent.Children = append(parent.Children, &ErrorNode{
				Error:   newErr,
				Handler: handler,
				Parent:  parent,
			})
			continue
		}
		// if node is a father of another one
		if chidx, children := findChildren(newErr); len(children) > 0 {
			newNode := &ErrorNode{
				Error:    newErr,
				Handler:  handler,
				Children: children,
			}
			removeRoots(chidx)
			errorTree = append(errorTree, newNode)
			continue
		}
		// otherwise as a new node
		errorTree = append(errorTree, &ErrorNode{
			Error:   newErr,
			Handler: handler,
		})
	}
	return nil
}

// UseGlobal registers global middlewares, which will be executed
// before the local middlewares and local handlers
func UseGlobal(middlewares ...ErrorMiddleware) {
	globalMiddlewares = MiddlewareChain(middlewares...)
}

// UseWithMiddleware is a shortcut for Use()
func UseWithMiddleware(middleware ErrorMiddleware, handler ErrorHandler, errs ...error) error {
	return Use(func(err error, ctx *gin.Context) {
		middleware(handler)(err, ctx)
	}, errs...)
}

// MiddlewareChain wraps middlewares into a single ErrorMiddleware
func MiddlewareChain(middlewares ...ErrorMiddleware) ErrorMiddleware {
	return func(eh ErrorHandler) ErrorHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			eh = middlewares[i](eh)
		}
		return eh
	}
}

// Chain wraps handlers into a single ErrorHandler
func Chain(handlers ...ErrorHandler) ErrorHandler {
	return func(err error, ctx *gin.Context) {
		for _, handler := range handlers {
			handler(err, ctx)
		}
	}
}

// ErrbinMiddleware return a gin.HandleFunc as a middleware
// and handle the last *gin.Error in *gin.Context.
// If no such handler exists, then the fallbackHandler will be execute.
func ErrbinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		h, found := findHandler(err)
		if !found {
			h = fallbackHandler
		}
		globalMiddlewares(h)(err, c)
	}
}

// Fallback allows to set a customize default/fallback ErrorHandler
func Fallback(fn ErrorHandler) {
	fallbackHandler = fn
}

func findPosition(target error) (*ErrorNode, *ErrorNode) {
	var trave func(nodes []*ErrorNode) (*ErrorNode, *ErrorNode)
	trave = func(nodes []*ErrorNode) (*ErrorNode, *ErrorNode) {
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

func findChildren(newErr error) (chidx []int, children []*ErrorNode) {
	for i := len(errorTree) - 1; i >= 0; i-- {
		root := errorTree[i]
		if errors.Is(root.Error, newErr) {
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
		return itself.Handler, true
	} else if parent != nil {
		return parent.Handler, true
	} else {
		return nil, false
	}
}
