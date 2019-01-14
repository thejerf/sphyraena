package router

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/sphyrw"
	"github.com/thejerf/sphyraena/sphyrw/hole"
)

// FIXME: Probably needs to live somewhere else

var ErrStreamHandlerNotFound = errors.New("stream handler not found")

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

// RunRoute runs the given route with an HTTP request (not a streaming request).
func (sr *SphyraenaRouter) RunRoute(rw *sphyrw.SphyraenaResponseWriter, req *request.Request) {
	handler, routeResult, err := sr.getHTTPHandler(req)
	if err != nil {
		// FIXME: need to do something different
		panic(err.Error())
	}

	// This means that if a nil handler is returned, any content in the
	// routeResult is ignored, which is probably good from a security POV.
	// Though as SphyRW gets stronger and starts returning things, maybe
	// that will be less true.
	if handler == nil {
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

	handler.ServeStreaming(rw, req)
}

func (sr *SphyraenaRouter) RunStreamingRoute(req *request.Request) {
	// FIXME: This MUST handle panics! It's being run as a top-level goroutine.
	handler, routeResult, err := sr.getStreamingHandler(req)
	if err != nil || handler == nil {
		if err != nil {
			fmt.Println("Error getting the streaming handler:", err)
		}
		req.StreamResponse(request.StreamRequestResult{
			Error:     ErrStreamHandlerNotFound.Error(),
			ErrorCode: 404,
		})
		return
	}

	req.RouteResult = routeResult
	// apply security holes here?

	fmt.Println("Using handler:", handler)
	handler.HandleStream(req)

	// FIXME: If we get here and no stream was opened we should emit an
	// error to the initial response handler.
}

// this is primarily broken out for the tests
func (sr *SphyraenaRouter) getHTTPHandler(req *request.Request) (request.Handler, *request.RouteResult, error) {
	routerRequest := newRequest(req)

	result := sr.Route(routerRequest)

	if result.Handler == nil {
		return nil, nil, nil
	}

	return result.Handler, routerRequest.routeResult(), result.Error
}

func (sr *SphyraenaRouter) getStreamingHandler(req *request.Request) (
	request.StreamHandler,
	*request.RouteResult,
	error,
) {
	routerRequest := newRequest(req)
	result := sr.Route(routerRequest)
	spew.Dump("stream route result:", result)
	if result.StreamHandler == nil {
		return nil, nil, nil
	}

	return result.StreamHandler, routerRequest.routeResult(), result.Error
}
