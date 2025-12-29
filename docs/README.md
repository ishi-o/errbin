# Errbin

Declarative error handling middleware for Gin with hierarchical error matching and middleware chaining.

English | [中文](/docs/README.zh-CN.md)

## Features

- **Hierarchical Error Matching**: Handle errors and all their descendant types
- **Middleware Support**: Chain middleware for cross-cutting concerns
- **Declarative API**: Clean, type-safe API for defining error handlers
- **Error Inheritance**: Automatically matches derived errors
- **Zero Dependencies**: Only depends on Gin
- **Simple Integration**: Drop-in middleware for Gin applications

## Installation

```bash
go get github.com/ishi-o/errbin
```

## Quick Start

Errbin allows you to define handlers for specific error types and their subtypes in a declarative way.

### Basic Usage

1. Register error handlers during application initialization

   ```go
   import (
   	"github.com/gin-gonic/gin"
   	"github.com/ishi-o/errbin"
   )

   func init() {
   	errbin.Use(func(err error, c *gin.Context) {
   		c.JSON(404, gin.H{"error": "Not found"})
   	}, ErrNotFound)

   	errbin.Use(func(err error, c *gin.Context) {
   		c.JSON(401, gin.H{"error": "Unauthorized"})
   	}, ErrUnauthorized)

   	r.Use(errbin.ErrbinMiddleware())
   }
   ```

2. Add Errbin middleware to your Gin router

   ```go
   import (
   	"log"

   	"github.com/gin-gonic/gin"
   	"github.com/ishi-o/errbin"
   )

   func init() {
   	errbin.UseGlobal(func(next errbin.ErrorHandler) errbin.ErrorHandler {
   		return func(err error, c *gin.Context) {
   			log.Printf("Error: %v", err)
   			next(err, c)
   		}
   	})

   	errbin.Use(func(err error, c *gin.Context) {
   		c.JSON(500, gin.H{"error": "Database error"})
   	}, ErrDatabase)

   	r.Use(errbin.ErrbinMiddleware())
   }
   ```

3. Let Errbin automatically match and handle errors

### Core Concepts

- **Error Hierarchy**: Handlers match errors using Go's `errors.Is()` semantics
- **ErrorHandler**: Functions that process errors and generate HTTP responses
- **ErrorMiddleware**: Wrappers that add functionality like logging or metrics
- **Global Middleware**: Middleware applied to all error handlers
- **Fallback Handler**: Default handler for unhandled errors

## API Overview

### Core Functions

- `Use()` - Register an error handler for specific error types
- `UseGlobal()` - Register global middleware for all handlers
- `ErrbinMiddleware()` - Gin middleware that processes errors
- `Fallback()` - Set custom fallback handler for unhandled errors

### Middleware Utilities

- `MiddlewareChain()` - Chain multiple middleware together
- `UseWithMiddleware()` - Register handler with middleware
- `Chain()` - Chain multiple handlers (executed in order)

## Important Notes

- **Initialization Only**: `Use()` must be called during application initialization, not after the server starts
- **Non-Concurrent**: Error registration is not thread-safe and should happen before server startup
- **Error Inheritance**: Handlers for parent errors will also handle all child errors
- **Fallback Handler**: Always set a meaningful fallback for unexpected errors

## License

MIT License - see [LICENSE](/LICENSE)

## Acknowledgments

Built for the [Gin Web Framework](https://github.com/gin-gonic/gin), inspired by hierarchical error handling patterns.
