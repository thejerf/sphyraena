package request

import "github.com/thejerf/sphyraena/identity/auth/enticate"

// A type used to load the context with authentication-related values.
type authenticationKey struct{}

// SetAuthError sets the given error as the AuthError for the current web
// page request.
func (c *Request) SetAuthError(err enticate.AuthError) {
	c.Set(authenticationKey{}, err)
}

// ValueAuthError returns the AuthError as a correctly-typed value, or nil if there
// is no AuthError.
func (c *Request) ValueAuthError() enticate.AuthError {
	val := c.Value(authenticationKey{})
	if val == nil {
		return nil
	}
	return val.(enticate.AuthError)
}
