package ginutils

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	bhttp "github.com/brynbellomy/go-utils/http"
)

type ginContextKey struct{}

func init() {
	bhttp.SetParamExtractor(func(r *http.Request, param string) string {
		ginCtx, ok := r.Context().Value(ginContextKey{}).(*gin.Context)
		if !ok {
			return ""
		}
		return ginCtx.Param(param)
	})

	bhttp.SetContextExtractor(func(ctx context.Context, key string) any {
		ginCtx, ok := ctx.Value(ginContextKey{}).(*gin.Context)
		if !ok {
			panic("didn't set up middleware properly")
		}
		val, _ := ginCtx.Get(key)
		return val
	})
}

func GinContextUnmarshaler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), ginContextKey{}, c)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func UnmarshalGinRequest(into any, ginCtx *gin.Context) error {
	// Store gin context in http.Request
	ctx := context.WithValue(ginCtx.Request.Context(), ginContextKey{}, ginCtx)
	r := ginCtx.Request.WithContext(ctx)
	return bhttp.UnmarshalHTTPRequest(into, r)
}
