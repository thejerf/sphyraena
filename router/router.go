/*

Package router implements the router for sphyraena.

"There's a trillion routers for Go! Why didn't you just use one of those?"

As with many other things, the Sphyraena router is designed to work
with the idea of being default secure. This creates two major concerns.

First, auditability. Routers always look simple in the sample code, but
real routers tend to grow complicated, and it becomes very difficult to
understand what they are doing. You can also see this in things like
nginx or apache configuration, which are ultimately routers too. It
is difficult to simply read them and work out what they are doing.

Sphyraena routers are designed to be able to be fully configured through
a variety of convenient mechanisms, such as being able to modularly
include files, use code to configure the routes, etc., and at the end
of all the manipulation, the final routing table can be viewed online
view a convenient web page view that allows a team to fully audit exactly
what pages are going through what security gates. If there is any
confusion, default deny is applied. Work will be done to do things like
"show paths that require no auth".

The following paragraphs are aspirational and depend on determining a good
interface for the following code. (Perhaps I make you essentially 'marshal'
the data from an HTTP request into your type, then call a bare .Handle()
on your type? This would be especially effective if we can automatically
somehow create an easy path for marshaling basic forms or something and
doing the other basic work in some composable manner.)

Second, Sphyraena implements default deny. In this case this manifests as
filtering requests before they get to the actual handlers. HTTP requests
are very complicated things, and auditing the behavior of a handler can
be quite difficult with such complex input. By specifying precisely
what input a handler can receive, making the "default deny" being just
"a request was received", it makes it easier for a security auditor
to verify that crazy inputs won't make the function do something crazy
in its outputs. It is also, as it happens, just plain good software
engineering... code that takes in more constrained input is easier to
reuse than code that takes in "a huge complicated HTTP request".

Also, from a sheer feature perspective, many routers in Go operate only on
the URL path. This is fine for many use cases, but the application that
Sphyraena is being written for already has a rich set of existing routes
that have grown up over the years, many of which do things like take
querystring "routing" parameters or other arbitrarily crazy things. This
design permits and/or requires full examination of the incoming request.
This also allows for security assertions like validating that a request
is only available over HTTPS.

*/
package router

// Features:
// * host matching
// * tls matching
// * userrole matching (later)
// * capture:
//   * substrings up to /, typed
//   * rest of URL
//   * URL parameters
// * Explicit declaration of the incoming headers that the response
//   can deal with. This should be both leverageable for auditability
//   for security (especially for proxied requests), and also make it
//   easier to test these things by making it much clearer what the
//   requests can and can not contain.

// FIXME: Router needs to wrap the context and offer a Set/Value
// interface. Probably need to expose the Session via that mechanism too,
// so if the session is set but we somehow escape out of that router it
// won't "stick".

import (
	"net/http"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw/cookie"
	"github.com/thejerf/sphyraena/sphyrw/hole"
)

type RouterFrame struct {
	// The remaining path to be processed after this frame
	path       []byte
	parameters map[string]string
	headersAdd http.Header
	headersSet http.Header
	cookies    map[string]*cookie.OutCookie
	holes      []hole.SecurityHole
	consume    int
	isFinal    bool
}

func (rr *Request) routeResult() *context.RouteResult {
	parameters := map[string]string{}
	headers := http.Header{}
	cookies := map[string]*cookie.OutCookie{}
	holes := hole.SecurityHoles{}
	for _, frame := range rr.frames[0:rr.current] {
		for key, value := range frame.parameters {
			parameters[key] = value
		}
		for key, headerset := range frame.headersAdd {
			for _, value := range headerset {
				headers.Add(key, value)
			}
		}
		for key, headersset := range frame.headersSet {
			for _, value := range headersset {
				headers.Set(key, value)
			}
		}
		for name, cookie := range frame.cookies {
			cookies[name] = cookie
		}
		holes = append(holes, frame.holes...)
	}

	remainingPath := string(rr.frames[rr.current].remainingPath())
	precedingPath := string(rr.basePath[0 : len(rr.basePath)-len(remainingPath)])

	return &context.RouteResult{
		Parameters:    parameters,
		PrecedingPath: precedingPath,
		RemainingPath: remainingPath,
		Headers:       headers,
		Cookies:       cookies,
		Holes:         holes,
	}
}

// A request is finalized either when the matcher says it is, or when the
// entire path has been consumed by something.
func (rr *Request) requestComplete() bool {
	if rr.frames[rr.current].isFinal {
		return true
	}
	return len(rr.frames[rr.current].path) == rr.frames[rr.current].consume
}

func (rf *RouterFrame) Reset(path []byte) {
	rf.headersAdd = http.Header{}
	rf.headersSet = http.Header{}
	rf.holes = rf.holes[:0]
	rf.cookies = map[string]*cookie.OutCookie{}
	rf.path = path
	rf.parameters = nil
}

// A Request is a context.Context for the current request, gussied
// up with some additional things that allow it to track the process of
// routing a request to the target handler.
//
// Request should generally not be *modified*, for reasons of software
// complexity more than anything else. However it could at times end up
// swapped out wholesale.
//
// The Request indirects many of the Context operations such as setting
// values, setting headers, and other such things, and arranges it so that
// only Clauses that are actually on the path to the final response will
// have an effect on the final request. That is to say, if a clause is
// recursed into, and something inside that recursion sets a header, but
// then the routing fails out of that clause, the header will not be
// set. Everything is isolated.
type Request struct {
	basePath []byte
	frames   []RouterFrame
	holes    []hole.SecurityHole
	current  int
	limit    int
	*context.Context
}

func (rf *RouterFrame) remainingPath() []byte {
	// FIXME: need checks
	return rf.path[rf.consume:]
}

func newRequest(ctx *context.Context) *Request {
	frames := make([]RouterFrame, 5)
	basePath := []byte(ctx.URL.Path)
	frames[0].path = basePath
	return &Request{
		basePath: basePath,
		frames:   frames,
		holes:    []hole.SecurityHole{},
		current:  0,
		Context:  ctx,
	}
}

func (rr *Request) AddParameter(key, value string) {
	currentFrame := &rr.frames[rr.current]
	if currentFrame.parameters == nil {
		currentFrame.parameters = map[string]string{}
	}
	currentFrame.parameters[key] = value
}

func (rr *Request) AddSecurityHole(hole hole.SecurityHole) {
	rr.holes = append(rr.holes, hole)
}

// AddHeader adds an HTTP header to the response only if this frame is used
// in the final routing request.
func (rr *Request) AddHeader(key, value string) {
	rr.frames[rr.current].headersAdd.Add(key, value)
}

// AddCookie adds a cookie to this request, using the current session as
// the Authenticator. It mirrors cookie.NewOut
func (rr *Request) AddCookie(c *cookie.OutCookie) {
	rr.frames[rr.current].cookies[c.Name()] = c
}

// SetHeader sets the given HTTP header in the response only if this frame
// is used in the final routing request.
func (rr *Request) SetHeader(key, value string) {
	rr.frames[rr.current].headersSet.Set(key, value)
}

func (rr *Request) ConsumePath(c int) {
	rr.frames[rr.current].consume += c
}

func (rr *Request) ConsumeEntirePath() {
	curFrame := &rr.frames[rr.current]
	curFrame.consume = len(curFrame.path)
}

func (rr *Request) PathConsumed() []byte {
	consumed := 0
	for _, frame := range rr.frames[0 : rr.current+1] {
		consumed += frame.consume
	}
	return rr.basePath[0:consumed]
}

func (rr *Request) Finalize() {
	rr.frames[rr.current].isFinal = true
}

func (rr *Request) advance() error {
	prevFrame := rr.frames[rr.current]
	rr.current++
	rr.frames = append(rr.frames, RouterFrame{})
	rr.frames[rr.current].Reset(prevFrame.remainingPath())
	rr.frames[rr.current].isFinal = prevFrame.isFinal
	return nil
}

func (rr *Request) retreat() {
	// by construction, this won't go negative
	rr.current--
}

// CurrentPath returns the currently-remaining path under consideration.
//
// Callers MUST NOT modify the []byte. Rewriting the path must be done via
// ChangePath (not yet implemented).
func (rr *Request) CurrentPath() []byte {
	return rr.frames[rr.current].path
}

// A Router is something that can participate in the routing of the
// request. The Result contains the result of the given route request.
//
// If this returns a non-nil context.Handler, that will be the result of this
// call. Router terminates.
//
// If the context.Handler is nil, but the Router is non-nil, then this Router
// will be recursed into, counting against the recursion limit. It
// otherwise behaves normally.
//
// If an error is returned, processing for the current enclosing RouteBlock
// terminates. If errors are being logged, it will be logged.
//
// If all of these things are nil, processing will simply continue onwards.
type Router interface {
	Route(*Request) Result
}

// A Result is the result of calling a Route operation.
type Result struct {
	context.Handler
	*RouteBlock
	Error error
}

var emptyResult = Result{nil, nil, nil}

// A RouterClause is something that can route, and also has sufficient
// metadata to reconstruct itself when being audited or serialized.
//
// A RouterClause is allowed to return a RouteBlock, but it can only have
// one, and it must be constant.
type RouterClause interface {
	Router

	// Name() is the name that will be used in the routing configuration to
	// create a clause of this type. This should be a "class method" that
	// returns a constant string.
	Name() string

	// The "Argument" for a given clause is the string that, when used in
	// the configuration file, recreates this router.
	Argument() string

	// RouteBlock returns the constant route block the RouterClause can
	// route the request down. It must be constant for a given RouterClause
	// once serving starts.
	//
	// "nil" is distinct from an empty RouteBlock. nil means the
	// RouterClause never takes a RouteBlock.
	GetRouteBlock() *RouteBlock

	// This should return an empty value of the correct type for
	// serialization or deserialization via (TBD serialization).
	Prototype() RouterClause
}

// A RouteBlock is simply a collection of Routers, which can also be used
// as a Router.
//
// While some convenience methods are included, it is important to point
// out that this is all fully public on purpose. You are allowed to
// construct this in any manner you see fit, as long as you don't modify it
// once serving starts.
type RouteBlock struct {
	clauses []RouterClause
}

// Route implements the Route method on a RouteBlock.
//
// As a special case, a call to this method will never itself yield a
// non-nil *RouteBlock.
func (rb *RouteBlock) Route(rr *Request) Result {
	// expanding on the second paragraph out of view of the public docs: it's
	// important that RouteBlocks do not yield non-nil RouteBlocks of their
	// own, because it breaks the recursion below. It's also important from
	// a type-system point of view that we can guarantee no non-nil returns
	// from RouteBlocks because Sphyraena's internals use that guarantee
	// themselves; for instance the SphyraenaRouter has a RouteBlock and
	// not a Router for that very reason. By having "RouteBlock"s instead
	// of the generic Router we can get these guarantees where we need them.
	//
	// This can be verified by observing that all returns in this function
	// have nil in the second param.

	rr.advance()

	for _, router := range rb.clauses {
		res := router.Route(rr)
		if res.Handler != nil {
			return res
		}
		if res.RouteBlock != nil {
			res2 := res.RouteBlock.Route(rr)
			if res2.Handler != nil {
				return res2
			}
			// by construction, RouteBlocks never return more RouteBlocks
			if res2.RouteBlock != nil {
				panic("RouteBlock.Route return a non-nil RouteBlock")
			}
			if res2.Error != nil {
				return res2
			}
		}
		if res.Error != nil {
			return res
		}
	}

	// NOT deferred above on purpose; the "advance"s without corresponding
	// "retreat"s represent the actual path taken
	rr.retreat()

	// Guess this block doesn't apply/do anything useful.
	return emptyResult
}

// A convenience function for creating new RouteBlocks.
func NewRouteBlock(clauses ...RouterClause) *RouteBlock {
	return &RouteBlock{clauses}
}

// Add is a simple convenience function for adding to a RouteBlock.
func (rb *RouteBlock) Add(c ...RouterClause) {
	rb.clauses = append(rb.clauses, c...)
}

// AddLocation is a simple convenience function for adding a static
// RouteBlock to a given path.
func (rb *RouteBlock) AddLocation(path string, rrb *RouteBlock) {
	rb.Add(&StaticLocation{path, rrb})
}

// Location adds a new StaticLocation element and returns the resulting
// RouteBlock for further modification.
func (rb *RouteBlock) Location(path string) *RouteBlock {
	rrb := NewRouteBlock()
	rb.Add(&StaticLocation{path, rrb})
	return rrb
}

// AddLocationReturn is a simple convenience function to add a
// streaming REST handler directly to the given location.
func (rb *RouteBlock) AddLocationReturn(path string, h context.Handler) {
	rb.Add(&StaticLocation{path, DirectReturn(h)})
}

// AddLocationForward is a simple convenience function to add a
// ForwardClause directly to a given location.
func (rb *RouteBlock) AddLocationForward(path string, h context.Handler) {
	rb.Add(&StaticLocation{path, &RouteBlock{[]RouterClause{ForwardClause{h}}}})
}

// This implements the RouterClause interface in such a way that if you
// embed a *RouteBlock directly into your RouterClause's type, this
// automatically implements the RouteBlock method.
func (rb *RouteBlock) GetRouteBlock() *RouteBlock {
	return rb
}

func DirectReturn(h context.Handler) *RouteBlock {
	return &RouteBlock{[]RouterClause{ReturnClause{h}}}
}
