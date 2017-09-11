package context

// FIXME: This package needs a name other than context, goimports keeps
// resolving this to the google version.

import (
	"fmt"
	"net/http"
	"time"

	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/sphyrw"
	"github.com/thejerf/sphyraena/sphyrw/cookie"
	"github.com/thejerf/sphyraena/sphyrw/hole"
	"github.com/thejerf/sphyraena/strest"
)

type SphyraenaState struct {
	session.SessionServer

	// This is the default object to use for an unauthenticated user
	// If nil, this will automatically be set to enticate.DefaultUnauthenticated{}.
	defaultIdentity func() *identity.Identity
}

// development notes:
// it would be nice if the context couldn't be meaningfully written to by
// RoutingClauses, since that's pretty much an error waiting to happen. As
// I write this, a casual examination of the Context suggests that the only
// thing that you can write to is the google Context elements, which
// suggests we might be able to wrap this simply via a conversion into a
// type that doesn't offer that functionality. Currently waiting-and-seeing
// to see what all the Context ends up with before it's all said and done.

// Context is the Sphyraena-specific context for requests.
//
// This also complies with the interface for contexts as defined at
// https://godoc.org/golang.org/x/net/context#Context (or whereever that
// may live in the future, if it gets into core). As of this writing, full
// functionality is not yet supported, but the interface is conformed to.
type Context struct {
	*SphyraenaState
	*RouteResult
	session session.Session
	Cookies *cookie.InCookies

	*http.Request

	values map[interface{}]interface{}

	currentStream *strest.Stream

	// hack for now. Making it something public the user can screw with is
	// a code smell. FIXME this ought to come in the form of providing a
	// streaming context.
	// FIXME It's even worse than that because if we want to reuse this
	// context for subrequests, this isn't necessarily safe to pass
	// along. Still, as it says, hack.
	RunningAsGoroutine bool
}

// Session retrieves the current session for the session.
func (c *Context) Session() session.Session {
	return c.session
}

// SetSession sets the session for the current context.
//
// Doing this will automatically .Expire the current session. This is for
// security reasons; sessions should be renewed after every session change
// (see
// https://www.owasp.org/index.php/Session_Management_Cheat_Sheet#Renew_the_Session_ID_After_Any_Privilege_Level_Change ).
// It is intended that any attempt to modify the user's permissions
// requires a new session and results in a new session authorization
// (cookie, usually).
func (c *Context) SetSession(s session.Session) {
	// note this does not manipulate cookies, because there are session
	// mechanims other than cookies.
	c.session.Expire()
	c.session = s
}

// A Sphyraena-specific context key type.
type Key string

// This is the specific context generated by the routing.
type RouteResult struct {
	// These are the capture parameters for the request, the path that
	// matched, and the remaining unmatched path for the request
	Parameters    map[string]string
	Cookies       map[string]*cookie.OutCookie
	Headers       http.Header
	PrecedingPath string
	RemainingPath string
	Holes         hole.SecurityHoles
}

// Deadline implements the Context's Deadline method, by hardcoding that there
// is no deadline.
func (c *Context) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Done implements the Context's Done method by always returning nil.
func (c *Context) Done() <-chan struct{} {
	return nil
}

// Err implements the Context's Err method by always returning nil.
func (c *Context) Err() error {
	return nil
}

// Value returns the value the context contains for the given key. Keys
// reserved by Sphyraena itself use Sphyraena-specific types. You should
// use only your own types or built-in types as keys.
func (c *Context) Value(key interface{}) interface{} {
	return c.values[key]
}

func (c *Context) Set(key, value interface{}) {
	c.values[key] = value
}

func NewSphyraenaState(ss session.SessionServer, defaultIdentity func() *identity.Identity) *SphyraenaState {
	if defaultIdentity == nil {
		defaultIdentity = func() *identity.Identity {
			return identity.AnonymousIdentity
		}
	}

	return &SphyraenaState{
		SessionServer:   ss,
		defaultIdentity: defaultIdentity,
	}
}

// FIXME: May be the wrong name now as it grows.
// FIXME: Yes, there's a huge mess developing here between the context and
// the SPHYRW.

func (ss *SphyraenaState) NewContext(rw http.ResponseWriter, req *http.Request) (*Context, *sphyrw.SphyraenaResponseWriter) {
	// For now, put all requests into the same session
	var failedCookies []string
	cookies, failedCookies := cookie.ParseCookies(req.Header["Cookie"],
		ss.SessionServer)
	srw := sphyrw.NewSphyraenaResponseWriter(rw)

	if len(failedCookies) != 0 {
		// temporary for debugging
		fmt.Printf("Rejecting cookies: %v\n", failedCookies)
		for _, cookieName := range failedCookies {
			cookie, err := cookie.NewNonstandardOut(cookieName, "", nil, cookie.Delete)
			if err != nil {
				continue
			}
			srw.SetCookie(cookie)
		}
	}

	return &Context{
		SphyraenaState: ss,
		Request:        req,
		session:        session.AnonymousSession,
		Cookies:        cookies,
		values:         map[interface{}]interface{}{},
	}, srw
}
