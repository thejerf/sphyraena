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
	Options               []func(*cookie.OutCookie) error
}

type justAuthenticated struct{}

func markJustAuthenticated(c *context.Context) {
	c.Set(justAuthenticated{}, true)
}

// IsJustAuthenticated returns true if the passed-in context represents a
// request where the user was password authenticated. That is, not just
// that they were authenticated via the cookie, but that they logged in
// with a username and password.
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
func NewCookieAuth(rb *router.RouteBlock, pa enticate.PasswordAuthenticator) (*CookieAuth, error) {
	if rb == nil {
		return nil, errors.New("No router block passed in for cookie auth")
	}
	if pa == nil {
		return nil, errors.New("no password authenticator passed in for cookie auth")
	}
	return &CookieAuth{rb, pa, nil}, nil
}

func (ca *CookieAuth) Route(r *router.Request) (res router.Result) {
	sessionCookie := r.Context.Cookies.Get("session")

	if sessionCookie == nil {
		// FIXME: CSRF form protection
		r.ParseForm()

		username := unicode.NFKCNormalize(r.Form.Get("username"))
		password := unicode.NFKCNormalize(r.Form.Get("password"))

		auth, err := ca.passwordAuthenticator.Authenticate(username, password)
		if auth != nil {
			identity := &identity.Identity{auth}
			session, err := r.NewSession(identity)
			if err != nil {
				fmt.Printf("What does it mean for this error: %v\n", err)
				// FIXME
				res.RouteBlock = ca.authBlock
				return
			}
			r.SetSession(session)
			markJustAuthenticated(r.Context)
			hasID, sessionID := session.SessionID()
			if hasID {
				err := r.AddCookie("session", string(sessionID), ca.Options...)
				if err != nil {
					res.Error = err
					return
				}
				// deliberately pass through to subsequent resources
				return
			} else {
				fmt.Printf("Established session without identity?\n")
			}
		} else if err != nil {
			fmt.Printf("Got an auth error: %v\n", err)
			r.Context.SetAuthError(err)
			res.RouteBlock = ca.authBlock
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
