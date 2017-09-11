package enticate

import (
	"errors"

	"github.com/thejerf/sphyraena/unicode"
)

// this needs to use the class method pattern to create serializable
// authentications for use in sessions.

// ErrNoUniqueAuthenticationID is returned by authentication types that do
// not uniquely identify users, such as defaultUnauthenticated, when asked
// for their UniqueID.
var ErrNoUniqueAuthenticationID = errors.New("no unique authentication ID")

// DefaultUnauthenticated is a default authentication containing no
// authentication information.
var DefaultUnauthenticated Authentication = defaultUnauthenticated{}

// Authentication is the base interface that all objects that provide
// Authentication must conform to.
type Authentication interface {
	// When logging something about a request made with this
	// authentication, this is how to identify the target authentication in
	// the log. This may be a username, an email, whatever makes sense
	// in your local system to serve as an identification. This should be
	// unique for a given authentication, if you don't want your log
	// messages to be nonsensical.
	LogName() string

	// This returns true if this authentication represents some sort of
	// positive authentication. A request being made by an unknown source,
	// which is to say, an unauthenticated source, should have an
	// authentication object that returns false for this.
	IsAuthenticated() bool

	// For any two Authentications (even between classes) with the same
	// UniqueID, it should represent the "same" authentication for your
	// system.
	UniqueID() (string, error)
}

// The DefaultUnauthenticated struct provides a default Authentication that
// corresponds to a user who has not had any authentication provided.
//
// If no UnauthenticatedUser object is given in Sphyraena's configuration,
// Sphyraena will use this as the unauthenticated user.
type defaultUnauthenticated struct{}

// LogName implements the Authentication interface.
//
// This always returns "Unauthenticated User".
func (u defaultUnauthenticated) LogName() string {
	return "Unauthenticated User"
}

// IsAuthenticated implements the Authentication interface.
//
// This always returns false.
func (u defaultUnauthenticated) IsAuthenticated() bool {
	return false
}

// UniqueID always returns nil, ErrNoUniqueAuthenticationID.
func (u defaultUnauthenticated) UniqueID() (string, error) {
	return "", ErrNoUniqueAuthenticationID
}

// A NamedUser implements the minimal Authentication interface. The given
// username is used as the return value from LogName.
type NamedUser struct {
	Username unicode.NFKCNormalized
}

// LogName implements the Authentication interface.
//
// This returns the Username of the NamedUser.
func (nu *NamedUser) LogName() string {
	return nu.Username.String()
}

// IsAuthenticated implements the Authenication interface. This returns
// true.
func (nu *NamedUser) IsAuthenticated() bool {
	return true
}

// UniqueID returns the Username of the given user.
func (nu *NamedUser) UniqueID() (string, error) {
	return nu.Username.String(), nil
}
