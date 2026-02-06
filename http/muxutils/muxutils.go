package muxutils

import (
	"net/http"

	"github.com/gorilla/mux"

	bhttp "github.com/brynbellomy/go-utils/http"
)

func init() {
	bhttp.SetParamExtractor(func(r *http.Request, param string) string {
		return mux.Vars(r)[param]
	})
}
