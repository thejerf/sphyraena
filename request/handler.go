package request

// This doesn't seem to belong in here necessarily, but it can't go in
// strest due to import loops. FIXME hopefully it will become clear where
// this belongs.

import (
	"net/http"

	"github.com/thejerf/sphyraena/sphyrw"
)

// A Handler is something that can handle Streaming REST responses.
//
// ServeStreaming is the core interface that permits Streaming REST
// responses. You should also consider implementing the MayStream interface
// on anything that implements this interface.
type Handler interface {
	ServeStreaming(*sphyrw.SphyraenaResponseWriter, *Request)
}

// A HandlerFunc allows a simple function to function as a Streaming REST
// handler, just like http.HandlerFunc.
type HandlerFunc func(*sphyrw.SphyraenaResponseWriter, *Request)

// ServeStreaming simply calls the HandlerFunc.
func (fh HandlerFunc) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, context *Request) {
	fh(rw, context)
}

// NetHTTPHandler is a wrapper around conventional http.Handlers. They will
// not be able to use Sphyraena's extra functionality, of course, but they
// will still be affected by header controls, etc.
type NetHTTPHandler struct {
	http.Handler
}

// Handle calls the handling method of the HTTP handler.
func (nhh NetHTTPHandler) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, context *Request) {
	nhh.ServeHTTP(rw, context.Request)
}
