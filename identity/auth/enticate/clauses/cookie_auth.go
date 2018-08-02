package clauses

import (
	"errors"
	"fmt"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/auth/enticate"
	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/router"
	"github.com/thejerf/sphyraena/sphyrw/cookie"
	"github.com/thejerf/sphyraena/unicode"
)

// FIXME: We need some way to guarantee that this won't be proceeded past
// if auth fails. Router at teh moment is awfully forgiving if we manage to
// get past this.

// CookieAuth is a router clause that turns on the cookie-based
// authentication. This allows you to not incur the costs of authentication
// on requests that don't need it.
//
// Options can be used to modify the cookie's options on the way out. This
// would probably be used primarily to add cookie.Insecure to the options
// to permit use on non-HTTPS environments.
type CookieAuth struct {
	authBlock             *router.RouteBlock
	passwordAuthenticator enticate.PasswordAuthenticator
	Options               []cookie.Option
}

type justAuthenticated struct{}

func markJustAuthenticated(c *context.Context) {
	c.Set(justAuthenticated{}, true)
}

// IsJustAuthenticated returns true if the passed-in context represents a
// request where the user was password authenticated. That is, not just
// that they were authenticated via the cookie, but that they logged in
// with a username and password.
//
// FIXME: Why is this? Should it just be as the name suggests and be
// IsJustAuthenticated, without regard to how the auth was done?
func IsJustAuthenticated(c *context.Context) bool {
	val := c.Value(justAuthenticated{})
	return val != nil && val.(bool)
}

// NewCookieAuth returns a new Cookie Authorization routing component. It
// is not possible to proceed past this without having authenticated.
//
// The provided RouteBlock will be used to determine how to authenticate an
// unauthenticated user.
//
// The PasswordAuthenticator is used to do the username/password
// authentication. If this check passes, the user will be Authenticated
// with the auth.Entication returned by the password authenticator.
//
// To use this as an inline blocker that requires authentication, you can
// pass in a RouteBlock that will perform authentication by providing the
// user a form. To use it with an independent REST request that will auth
// the user, you can pass in something that just statically returns some
// form of permission denied/404/whatever.
func NewCookieAuth(
	rb *router.RouteBlock,
	pa enticate.PasswordAuthenticator,
	options ...cookie.Option,
) (*CookieAuth, error) {
	if rb == nil {
		return nil, errors.New("No router block passed in for cookie auth")
	}
	if pa == nil {
		return nil, errors.New("no password authenticator passed in for cookie auth")
	}
	return &CookieAuth{rb, pa, options}, nil
}

// FIXME: CookieAdder belong here or somewhere else?
func PasswordAuthenticate(
	pa enticate.PasswordAuthenticator,
	r *context.Context,
	options ...cookie.Option,
) (*cookie.OutCookie, error) {
	// FIXME: CSRF form protection
	// FIXME: Which ideally shouldn't require a call here and/or can't be skipped
	r.ParseForm()

	username := unicode.NFKCNormalize(r.Form.Get("username"))
	password := unicode.NFKCNormalize(r.Form.Get("password"))

	auth, authErr := pa.Authenticate(username, password)
	if authErr != nil {
		fmt.Printf("Got an auth error: %v\n", authErr)
		r.SetAuthError(authErr)
		return nil, authErr
	}

	identity := &identity.Identity{auth}
	session, err := r.NewSession(identity)
	if err != nil {
		// FIXME
		fmt.Printf("What does it mean for this error: %v\n", err)
		return nil, err
	}
	r.SetSession(session)
	markJustAuthenticated(r)
	hasID, sessionID := session.SessionID()
	if hasID {
		cookie, err := cookie.NewOut(
			"session",
			string(sessionID),
			r.Session(),
			options...,
		)
		if err != nil {
			return nil, err
		}
		return cookie, nil
	} else {
		fmt.Printf("Established session without identity?\n")
	}

	return nil, nil
}

func (ca *CookieAuth) Route(r *router.Request) (res router.Result) {
	sessionCookie := r.Context.Cookies.Get("session")

	if sessionCookie == nil {
		cookie, err := PasswordAuthenticate(ca.passwordAuthenticator, r.Context)
		if err == nil {
			if cookie != nil {
				r.AddCookie(cookie)
			}
			// pass through to the underlying mechanism
			return
		}
		// If auth yielded neither an error nor an authentication, we are
		// probably visiting the page for the first time. We still need to
		// auth, but there is no error.
		res.RouteBlock = ca.authBlock
	} else {
		session, err := r.GetSession(session.SessionID(sessionCookie.Value()))
		if err != nil {
			// FIXME: This is actually an odd path, like, the session
			// expired between the cookie check and this extraction. Should
			// mark the session as expired or something and re-auth.
			res.RouteBlock = ca.authBlock
			return
		}
		r.SetSession(session)
		// Return with passthrough to subsequent resources
		return
	}

	return
}

func (ca *CookieAuth) Name() string {
	return "CookieAuth"
}

func (ca *CookieAuth) Argument() string {
	return ""
}

func (ca *CookieAuth) GetRouteBlock() *router.RouteBlock {
	return nil
}

func (ca *CookieAuth) Prototype() router.RouterClause {
	return &CookieAuth{}
}
