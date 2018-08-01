package identity

import (
	"errors"
	"strings"

	"github.com/thejerf/sphyraena/identity/auth/enticate"
)

var ErrInvalidIdentity = errors.New("invalid identity")

// FIXME: This sorta conflicts with the desire to put an Identity method on
// the Session, which ought to be satisfiable by composing in a single data type.

// An Identity bundles together all of the relevant Identity information
// into one composite struct.
type Identity struct {
	// FIXME: this really can't be composed in... it can't be public.
	enticate.Authentication
}

var AnonymousIdentity = &Identity{
	enticate.DefaultUnauthenticated,
}

func (i *Identity) Identity() *Identity {
	return i
}

// SetAuthentication sets the authentication for the given identity.
//
// FIXME: This doesn't happen yet, but it should also result in destroying
// any current session and looking up or creating a new one as needed.
func (i *Identity) SetAuthentication(enticate enticate.Authentication) {
	i.Authentication = enticate
}

func (i *Identity) MarshalText() ([]byte, error) {
	name, contents, err := enticate.Marshal(i.Authentication)
	if err != nil {
		return nil, err
	}

	return append([]byte(name+enticate.EnticateSeparator), contents...), nil
}

func (i *Identity) UnmarshalText(b []byte) error {
	split := strings.SplitN(string(b), enticate.EnticateSeparator, 2)
	if len(split) != 2 {
		return ErrInvalidIdentity
	}

	ty := split[0]
	authDetails := split[1]

	auth, err := enticate.Unmarshal(ty, []byte(authDetails))
	if err != nil {
		return err
	}
	i.Authentication = auth
	return nil
}
