package errbin

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestErrbin(t *testing.T) {
	resetGlobalState()

	var (
		ErrBase       = errors.New("base error")
		ErrSpecific   = fmt.Errorf("specific: %w", ErrBase)
		ErrMoreSpec   = fmt.Errorf("more specific: %w", ErrSpecific)
		ErrNotFound   = errors.New("not found error")
		ErrValidation = errors.New("validation error")
	)

	t.Run("Register base error", func(t *testing.T) {
		err := Use(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "base"})
		}, ErrBase)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(errorTree))
	})

	t.Run("Register specific error", func(t *testing.T) {
		err := Use(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "specific"})
		}, ErrSpecific)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(errorTree))
	})

	t.Run("Register leaf error", func(t *testing.T) {
		err := Use(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "more specific"})
		}, ErrMoreSpec)
		assert.NoError(t, err)
	})

	t.Run("Register duplicate error", func(t *testing.T) {
		err := Use(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "duplicate"})
		}, ErrBase)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate registration")
	})

	t.Run("Test error mapping", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.Error(ErrMoreSpec)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "more specific")

		r2 := gin.New()
		r2.Use(ErrbinMiddleware())
		r2.GET("/test2", func(c *gin.Context) {
			wrappedErr := fmt.Errorf("wrapped: %w", ErrSpecific)
			c.Error(wrappedErr)
		})

		w2 := httptest.NewRecorder()
		r2.ServeHTTP(w2, httptest.NewRequest("GET", "/test2", nil))
		assert.Contains(t, w2.Body.String(), "specific")

		r3 := gin.New()
		r3.Use(ErrbinMiddleware())
		r3.GET("/test3", func(c *gin.Context) {
			c.Error(errors.New("unknown error"))
		})

		w3 := httptest.NewRecorder()
		r3.ServeHTTP(w3, httptest.NewRequest("GET", "/test3", nil))
		assert.Equal(t, http.StatusInternalServerError, w3.Code)
	})

	t.Run("Test tree struct", func(t *testing.T) {
		assert.Equal(t, 1, len(errorTree))
		root := errorTree[0]
		assert.Equal(t, ErrBase, root.Error)
		assert.Equal(t, 1, len(root.Children))
		assert.Equal(t, ErrSpecific, root.Children[0].Error)
		assert.Equal(t, 1, len(root.Children[0].Children))
		assert.Equal(t, ErrMoreSpec, root.Children[0].Children[0].Error)
	})

	t.Run("Test UseGlobal", func(t *testing.T) {
		resetGlobalState()

		callLog := []string{}

		UseGlobal(
			func(next ErrorHandler) ErrorHandler {
				return func(err error, c *gin.Context) {
					callLog = append(callLog, "global-middleware-1")
					next(err, c)
				}
			},
			func(next ErrorHandler) ErrorHandler {
				return func(err error, c *gin.Context) {
					callLog = append(callLog, "global-middleware-2")
					c.Set("global_mw_called", true)
					next(err, c)
				}
			},
		)

		Use(func(err error, c *gin.Context) {
			callLog = append(callLog, "handler")
			c.JSON(404, gin.H{"error": "not found"})
		}, ErrNotFound)

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-global", func(c *gin.Context) {
			c.Error(ErrNotFound)
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test-global", nil))

		assert.Equal(t, 404, w.Code)
		assert.Equal(t, []string{"global-middleware-1", "global-middleware-2", "handler"}, callLog)
		assert.Contains(t, w.Body.String(), "not found")
	})

	t.Run("Test UseWithMiddleware", func(t *testing.T) {
		resetGlobalState()

		middlewareCalled := false
		handlerCalled := false

		err := UseWithMiddleware(
			func(next ErrorHandler) ErrorHandler {
				return func(err error, c *gin.Context) {
					middlewareCalled = true
					c.Set("custom_middleware", "executed")
					next(err, c)
				}
			},
			func(err error, c *gin.Context) {
				handlerCalled = true
				c.JSON(422, gin.H{"error": "validation failed", "details": err.Error()})
			},
			ErrValidation,
		)

		assert.NoError(t, err)

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-shortcut", func(c *gin.Context) {
			c.Error(ErrValidation)
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test-shortcut", nil))

		assert.True(t, middlewareCalled)
		assert.True(t, handlerCalled)
		assert.Equal(t, 422, w.Code)
		assert.Contains(t, w.Body.String(), "validation failed")
	})

	t.Run("Test MiddlewareChain", func(t *testing.T) {
		callOrder := []string{}

		m1 := func(next ErrorHandler) ErrorHandler {
			return func(err error, c *gin.Context) {
				callOrder = append(callOrder, "m1-before")
				next(err, c)
				callOrder = append(callOrder, "m1-after")
			}
		}

		m2 := func(next ErrorHandler) ErrorHandler {
			return func(err error, c *gin.Context) {
				callOrder = append(callOrder, "m2-before")
				c.Set("m2", "executed")
				next(err, c)
				callOrder = append(callOrder, "m2-after")
			}
		}

		m3 := func(next ErrorHandler) ErrorHandler {
			return func(err error, c *gin.Context) {
				callOrder = append(callOrder, "m3-before")
				next(err, c)
				callOrder = append(callOrder, "m3-after")
			}
		}

		chain := MiddlewareChain(m1, m2, m3)

		testHandler := func(err error, c *gin.Context) {
			callOrder = append(callOrder, "handler")
			c.JSON(200, gin.H{"status": "ok"})
		}

		chainedHandler := chain(testHandler)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		chainedHandler(nil, c)

		expectedOrder := []string{
			"m1-before", "m2-before", "m3-before",
			"handler",
			"m3-after", "m2-after", "m1-after",
		}
		assert.Equal(t, expectedOrder, callOrder)
	})

	t.Run("Test Chain", func(t *testing.T) {
		resetGlobalState()

		ErrChained := errors.New("chained error")
		callLog := []string{}

		h1 := func(err error, c *gin.Context) {
			callLog = append(callLog, "handler-1")
			c.Set("h1", "executed")
		}

		h2 := func(err error, c *gin.Context) {
			callLog = append(callLog, "handler-2")
			c.JSON(500, gin.H{"error": "internal server error"})
		}

		h3 := func(err error, c *gin.Context) {
			callLog = append(callLog, "handler-3")
		}

		chainedHandler := Chain(h1, h2, h3)

		Use(chainedHandler, ErrChained)

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-chain", func(c *gin.Context) {
			c.Error(ErrChained)
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test-chain", nil))
		assert.Equal(t, []string{"handler-1", "handler-2", "handler-3"}, callLog)
		assert.Equal(t, 500, w.Code)
		assert.Contains(t, w.Body.String(), "internal server error")
	})

	t.Run("Test Chain all handlers", func(t *testing.T) {
		resetGlobalState()

		callLog := []string{}

		h1 := func(err error, c *gin.Context) {
			callLog = append(callLog, "handler-1")
		}

		h2 := func(err error, c *gin.Context) {
			callLog = append(callLog, "handler-2")
		}

		h3 := func(err error, c *gin.Context) {
			callLog = append(callLog, "handler-3")
			c.JSON(200, gin.H{"status": "all handlers executed"})
		}

		chainedHandler := Chain(h1, h2, h3)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		chainedHandler(nil, c)

		assert.Equal(t, []string{"handler-1", "handler-2", "handler-3"}, callLog)
		assert.Equal(t, 200, w.Code)
		assert.Contains(t, w.Body.String(), "all handlers executed")
	})

	t.Run("Test Fallback", func(t *testing.T) {
		resetGlobalState()

		customFallbackCalled := false
		Fallback(func(err error, c *gin.Context) {
			customFallbackCalled = true
			c.JSON(418, gin.H{
				"error":    "I'm a teapot",
				"original": err.Error(),
			})
		})

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-fallback", func(c *gin.Context) {
			c.Error(errors.New("some unhandled error"))
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test-fallback", nil))

		assert.True(t, customFallbackCalled)
		assert.Equal(t, 418, w.Code)
		assert.Contains(t, w.Body.String(), "I'm a teapot")
		assert.Contains(t, w.Body.String(), "some unhandled error")
	})

	t.Run("Test Fallback with nil handler", func(t *testing.T) {
		resetGlobalState()

		customCalled := false
		Fallback(func(err error, c *gin.Context) {
			customCalled = true
			c.JSON(500, gin.H{"custom": true, "error": err.Error()})
		})

		Fallback(nil)

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-nil-fallback", func(c *gin.Context) {
			c.Error(errors.New("test error"))
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test-nil-fallback", nil))

		assert.True(t, customCalled)
		assert.Equal(t, 500, w.Code)
		assert.Contains(t, w.Body.String(), "custom")
		assert.Contains(t, w.Body.String(), "test error")
	})

	t.Run("Test Use with nil handler", func(t *testing.T) {
		resetGlobalState()

		err := Use(nil, ErrBase)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler cannot be nil")
	})

	t.Run("Test Use with nil error", func(t *testing.T) {
		resetGlobalState()

		err := Use(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "test"})
		}, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot register nil error")
	})

	t.Run("Test Use with multiple errors", func(t *testing.T) {
		resetGlobalState()

		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		err3 := errors.New("error 3")
		callCount := 0
		err := Use(func(err error, c *gin.Context) {
			callCount++
			c.JSON(400, gin.H{"msg": "multiple errors handler", "error": err.Error()})
		}, err1, err2, err3)
		assert.NoError(t, err)
		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-multi-1", func(c *gin.Context) {
			c.Error(err1)
		})
		r.GET("/test-multi-2", func(c *gin.Context) {
			c.Error(err2)
		})
		r.GET("/test-multi-3", func(c *gin.Context) {
			c.Error(err3)
		})
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, httptest.NewRequest("GET", "/test-multi-1", nil))
		assert.Equal(t, 400, w1.Code)

		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, httptest.NewRequest("GET", "/test-multi-2", nil))
		assert.Equal(t, 400, w2.Code)

		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, httptest.NewRequest("GET", "/test-multi-3", nil))
		assert.Equal(t, 400, w3.Code)
	})

	t.Run("Test UseGlobal without middlewares", func(t *testing.T) {
		resetGlobalState()

		testErr := errors.New("test error")
		Use(func(err error, c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		}, testErr)

		r := gin.New()
		r.Use(ErrbinMiddleware())
		r.GET("/test-no-global", func(c *gin.Context) {
			c.Error(testErr)
		})

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test-no-global", nil))

		assert.Equal(t, 200, w.Code)
		assert.Contains(t, w.Body.String(), "ok")
	})
}

func resetGlobalState() {
	errorTree = nil
	globalMiddlewares = nil
	fallbackHandler = func(err error, ctx *gin.Context) {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
	}
}
