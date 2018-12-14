package router

import (
	"net/http"

	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/sphyrw"
	"github.com/thejerf/sphyraena/sphyrw/hole"
)

// This package defines the top-level router that defines a Sphyraena
// application. It may someday come out of this module.
//
// FIXME: It looks like it could get its own module pretty easily. It's
// also possible some of this should like in sphyrw.

// A SphyraenaRouter is the top-level router for a Sphyraena
// application. It is responsible for enforcing many of the security
// guarantees that Sphyraena enforces.
type SphyraenaRouter struct {
	*RouteBlock

	sphyraenaState *request.SphyraenaState
}

func New(ss *request.SphyraenaState) *SphyraenaRouter {
	return &SphyraenaRouter{
		&RouteBlock{[]RouterClause{}},
		ss,
	}
}

// ServeHTTP implements the http.Handler interface.
func (sr *SphyraenaRouter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx, srw := sr.sphyraenaState.NewRequest(rw, req, false)

	sr.RunRoute(srw, ctx)
}

// this can be recursively called to re-run a route when something about
// the request has changed.
func (sr *SphyraenaRouter) RunRoute(rw *sphyrw.SphyraenaResponseWriter, req *request.Request) {
	streamingHandler, routeResult, err := sr.getStrest(req)
	if err != nil {
		// FIXME: need to do something different
		panic(err.Error())
	}

	// This means that if a nil handler is returned, any content in the
	// routeResult is ignored, which is probably good from a security POV.
	// Though as SphyRW gets stronger and starts returning things, maybe
	// that will be less true.
	if streamingHandler == nil {
		http.NotFound(rw, req.Request)
		return
	}

	req.RouteResult = routeResult
	for key, val := range routeResult.Headers {
		// safe because we only ever set values through the API
		rw.Header()[key] = val
	}
	for _, val := range routeResult.Cookies {
		rw.SetCookie(val)
	}
	// This can override a header set by the normal Header mechanism. This
	// is by design, because otherwise the routing table may be a
	// lie. (i.e., if the routing table says something has a given
	// protection applied but the handler overrides it, it becomes more
	// difficult to audit.) For this reason, the routing table is given
	// priority over the handlers.
	hole.ApplySecurityHeaders(rw.Header(), routeResult.Holes)

	// If either the handler, or the request, can't handle streaming, run
	// the request directly under the net/http handler as usual (no spawned
	// goroutine).
	mayStream, hasMayStream := streamingHandler.(request.MayStream)
	if (hasMayStream && mayStream.MayStream() == false) ||
		!req.CanHandleStream() {
		req.RunningAsGoroutine = false
		streamingHandler.ServeStreaming(rw, req)
		return
	}

	// If both the request and the handler can handle streaming, then go
	// ahead and do it.
	sr.runInGoroutine(streamingHandler, rw, req)
}

// this is primarily broken out for the tests
func (sr *SphyraenaRouter) getStrest(req *request.Request) (request.Handler, *request.RouteResult, error) {
	routerRequest := newRequest(req)

	result := sr.Route(routerRequest)

	return result.Handler, routerRequest.routeResult(), result.Error
}
