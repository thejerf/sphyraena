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

// MayStream is an interface that allows a Handler to declare in advance
// whether it is possible for this Handler to stream. If not, Sphyraena can
// optimize its response handling by not spawning a goroutine for the
// handler.
//
// Note if this code returns false, that constitutes a promise; if the
// Handler subsequently attempts to stream, it will get an error.
//
// (It is tempting to pass the context in here; I'd like to see an example
// of someone benchmarking that and showing a win in the general case
// before I add it, though. This is intended to just be a quick lil'
// optimization for things guaranteed not to stream, not a
// laboriously-computed completely accurate promise.)
type MayStream interface {
	MayStream() bool
}

// A HandlerFunc allows a simple function to function as a Streaming REST
// handler, just like http.HandlerFunc.
type HandlerFunc func(*sphyrw.SphyraenaResponseWriter, *Request)

// ServeStreaming simply calls the HandlerFunc.
func (fh HandlerFunc) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, context *Request) {
	fh(rw, context)
}

func (fh HandlerFunc) MayStream() bool {
	return true
}

// A HandlerFuncNoStream allows a simple function to function as a Streaming
// REST handler, just like http.HandlerFunc. It implements a false return
// for MayStream.
type HandlerFuncNoStream HandlerFunc

// MayStream implements the MayStream interface and returns false.
func (fh HandlerFuncNoStream) MayStream() bool {
	return false
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

// MayStream always returns false.
func (nhh NetHTTPHandler) MayStream() bool {
	return false
}
