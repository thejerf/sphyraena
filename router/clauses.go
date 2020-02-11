package router

import (
	"bytes"

	"github.com/thejerf/sphyraena/request"
)

// A StaticLocation matches a given static portion of the URL.
//
// Note that StaticLocation will only serve a resource if the location
// precisely matches. I've been bitten more times than I can count by
// code that has a route specified as "/a" and it successfully serves that
// when "/a_different_url" is requested. If you want matching behavior,
// see LocationForward.
type StaticLocation struct {
	Location    string `json:string`
	*RouteBlock `json:route_block`
}

// Route implements the RoutingClause interface.
func (sl *StaticLocation) Route(rr *Request) (res Result) {
	path := rr.CurrentPath()

	if bytes.HasPrefix(path, []byte(sl.Location)) {
		rr.ConsumePath(len(sl.Location))
		res.RouteBlock = sl.RouteBlock
	}

	return
}

func (sl *StaticLocation) Name() string {
	return "location"
}

func (sl *StaticLocation) Argument() string {
	return sl.Location
}

func (sl *StaticLocation) Prototype() RouterClause {
	return &StaticLocation{}
}

// A ReturnClause return a constant StRESTFactory, if the path is fully
// consumed.
//
// The latter clause is due to the number of times the author has been
// bitten by resources bound to "/a" that suddenly get routed to
// "/a_different_url", or worse, start stuffing XSS in there. If you want
// to forward along the remainder of the URL, use a Forwarding clause.
type ReturnClause struct {
	request.Handler
}

func (rc ReturnClause) Route(rr *Request) (res Result) {
	if len(rr.CurrentPath()) == 0 {
		res.Handler = rc.Handler
	}

	return
}

func (rc ReturnClause) Name() string {
	return "return"
}

func (rc ReturnClause) Argument() string {
	// TBD. StRESTFactories probably need to register themselves with a
	// name too, but that's out of scope right now
	return "TBD"
}

func (rc ReturnClause) GetRouteBlock() *RouteBlock {
	return nil
}

func (rc ReturnClause) Prototype() RouterClause {
	return ReturnClause{nil}
}

// A ForwardClause return a constant StRESTFactory, even if the path is
// not fully consumed, thus passing it along to the StREST.
//
// When writing routes, you should consider ReturnClauses the thing you use
// by default, until you find you need a ForwardClause.
//
// FIXME: Match documentation to the name of the handler.
type ForwardClause struct {
	request.Handler
}

func (rc ForwardClause) Route(*Request) (res Result) {
	res.Handler = rc.Handler
	return
}

func (rc ForwardClause) Name() string {
	return "forward"
}

func (rc ForwardClause) Argument() string {
	// TBD. StRESTFactories probably need to register themselves with a
	// name too, but that's out of scope right now
	return "TBD"
}

func (rc ForwardClause) GetRouteBlock() *RouteBlock {
	return nil
}

func (rc ForwardClause) Prototype() RouterClause {
	return ForwardClause{nil}
}

// A StreamClause returns a constant StreamingHandler, even if the path is
// not fully consumed, thus passing it along to the handler.
type StreamClause struct {
	request.StreamHandler
}

func (sc StreamClause) Route(*Request) (res Result) {
	res.StreamHandler = sc.StreamHandler
	return
}

func (sc StreamClause) Name() string {
	return "stream"
}

func (sc StreamClause) Argument() string {
	return ""
}

func (sc StreamClause) GetRouteBlock() *RouteBlock {
	return nil
}

func (sc StreamClause) Prototype() RouterClause {
	return StreamClause{}
}

// ExactLocation matches if and only if the location match is EXACT. This
// allows putting specific matches in front of things that will forward
// entire chunks of the URL space. This is useful for things like an index
// page, because otherwise a match on / will forward everything down this
// route block.
type ExactLocation struct {
	Location    string `json:string`
	*RouteBlock `json:route_block`
}

// Route implements the RoutingClause interface.
func (el *ExactLocation) Route(rr *Request) (res Result) {
	path := rr.CurrentPath()

	dprintln("Checking Exact Location", string(path), string(el.Location))
	if bytes.Equal(path, []byte(el.Location)) {
		rr.ConsumePath(len(el.Location))
		res.RouteBlock = el.RouteBlock
	}

	return
}

// Name return exact_location.
func (el *ExactLocation) Name() string {
	return "exact_location"
}

// Argument returns the arguments for the exact location.
func (el *ExactLocation) Argument() string {
	return el.Location
}

// Prototype returns an ExactLocation object.
func (el *ExactLocation) Prototype() RouterClause {
	return &ExactLocation{}
}

// AddExactLocation adds the given ExactLocation to a RouteBlock.
func AddExactLocation(rb *RouteBlock, path string, h request.Handler) {
	rb.Add(&ExactLocation{path, NewRouteBlock(ForwardClause{h})})
}
