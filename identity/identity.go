package identity

import "github.com/thejerf/sphyraena/identity/auth/enticate"

// FIXME: This sorta conflicts with the desire to put an Identity method on
// the Session, which ought to be satisfiable by composing in a single data type.

// An Identity bundles together all of the relevant Identity information
// into one composite struct.
type Identity struct {
	// this really can't be composed in... it can't be public.
	enticate.Authentication
}

var AnonymousIdentity = &Identity{
	enticate.DefaultUnauthenticated,
}

func (i *Identity) Idendity() *Identity {
	return i
}

// SetAuthentication sets the authentication for the given identity.
//
// FIXME: This doesn't happen yet, but it should also result in destroying
// any current session and looking up or creating a new one as needed.
func (i *Identity) SetAuthentication(enticate enticate.Authentication) {
	i.Authentication = enticate
}
