package enticate

import (
	"encoding"
	"errors"

	"github.com/thejerf/sphyraena/unicode"
)

func init() {
	Register(defaultUnauthenticated{})
	Register(&NamedUser{})
}

// this needs to use the class method pattern to create serializable
// authentications for use in sessions.

// ErrNoUniqueAuthenticationID is returned by authentication types that do
// not uniquely identify users, such as defaultUnauthenticated, when asked
// for their UniqueID.
var ErrNoUniqueAuthenticationID = errors.New("no unique authentication ID")

// ErrAuthenticationGiven is returned by the default unauthenticated user
// if you attempt to unmarshal it with any sort of actual content.
var ErrAuthenticationGiven = errors.New("authentication given for unauthenticated user")

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

	// This gives this sort of authentication type a name that can be used
	// by databases and such to uniquely identify it.
	AuthenticationName() string

	// Returns an empty instance of the given Authentication type that can
	// be used to unmarshal into.
	Empty() Authentication

	// The value returned by TextMarshaler should be unique for any given
	// user within the given authentication type.
	encoding.TextMarshaler

	// It must be valid to unmarshal anything that the TextMarshaler method
	// returned. For instance, even the default unauthenticated user
	// returns a value that accepts an empty []byte for unmarshaling, even
	// though it is impossible to unmarshal anything into that user.
	encoding.TextUnmarshaler
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

func (u defaultUnauthenticated) AuthenticationName() string {
	return "unauthenticated"
}

func (u defaultUnauthenticated) Empty() Authentication {
	return defaultUnauthenticated{}
}

func (u defaultUnauthenticated) MarshalText() ([]byte, error) {
	return []byte{}, nil
}

// UnmarshalText implements encoding.TextUnmarshaler on
// defaultUnauthenticated.
//
// The only legal things to be passed to this are either a nil or a
// 0-length []byte. Anything else will produce an error.
func (u defaultUnauthenticated) UnmarshalText(b []byte) error {
	// behold the rare legitimate Unmarshal method on a non-pointer!
	if len(b) == 0 {
		return nil
	}

	return ErrAuthenticationGiven
}

// GetNameUser provides a convenient method for getting a user name from a
// string.
//
// It is legal to directly construct NamedUsers, this is just a convenience.
func GetNamedUser(name string) *NamedUser {
	return &NamedUser{unicode.NFKCNormalize(name)}
}

// A NamedUser implements the minimal Authentication interface. The given
// username is used as the return value from LogName. It has no other state
// or content.
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

func (nu *NamedUser) AuthenticationName() string {
	return "simple_named_user"
}

func (nu *NamedUser) Empty() Authentication {
	return &NamedUser{}
}

func (nu *NamedUser) MarshalText() ([]byte, error) {
	return []byte(nu.Username.String()), nil
}

func (nu *NamedUser) UnmarshalText(b []byte) error {
	nu.Username = unicode.NFKCNormalize(string(b))
	return nil
}
