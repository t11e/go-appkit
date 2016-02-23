package appkit

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/context"
)

type ContextHandlerFunc func(
	ctx context.Context,
	w http.ResponseWriter,
	req *http.Request,
	params httprouter.Params)

func ContextizeHandler(ctx context.Context, fn ContextHandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		fn(ctx, w, req, params)
	}
}
