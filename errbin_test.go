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
	errorTree = nil

	var (
		ErrBase     = errors.New("base error")
		ErrSpecific = fmt.Errorf("specific: %w", ErrBase)
		ErrMoreSpec = fmt.Errorf("more specific: %w", ErrSpecific)
	)

	t.Run("Register base error", func(t *testing.T) {
		err := Register(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "base"})
		}, ErrBase)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(errorTree))
	})

	t.Run("Register specific error", func(t *testing.T) {
		err := Register(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "specific"})
		}, ErrSpecific)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(errorTree))
	})

	t.Run("Register leaf error", func(t *testing.T) {
		err := Register(func(err error, c *gin.Context) {
			c.JSON(400, gin.H{"msg": "more specific"})
		}, ErrMoreSpec)
		assert.NoError(t, err)
	})

	t.Run("Register duplicate error", func(t *testing.T) {
		err := Register(func(err error, c *gin.Context) {
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

		assert.Equal(t, http.StatusOK, w.Code)
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
		assert.Contains(t, w3.Body.String(), "Unhandled error")
	})

	t.Run("Test tree struct", func(t *testing.T) {
		assert.Equal(t, 1, len(errorTree))
		root := errorTree[0]
		assert.Equal(t, ErrBase, root.error)
		assert.Equal(t, 1, len(root.children))
		assert.Equal(t, ErrSpecific, root.children[0].error)
		assert.Equal(t, 1, len(root.children[0].children))
		assert.Equal(t, ErrMoreSpec, root.children[0].children[0].error)
	})
}
