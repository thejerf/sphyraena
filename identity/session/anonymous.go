package session

import (
	"errors"

	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/strest"
)

var anonymousIdentity *identity.Identity = identity.AnonymousIdentity

// MustSetAnonymousIdentity sets the *identity.Identity that will be
// returned by the Identity method of the AnonymousSession. The identity
// must be non-nil.
//
// This may only be set once. Any attempts to set it again will panic. The
// default is the identity.AnonymousIdentity.
func MustSetAnonymousIdentity(i *identity.Identity) {
	if i == nil {
		panic("Can't set anonymous identity to nil")
	}
	if anonymousIdentity != identity.AnonymousIdentity {
		panic("Can't set anonymous identity once it has already been set")
	}

	anonymousIdentity = i
}

// AnonymousSession is the session that is used by Sphyraena for users that
// are entirely unauthenticated. All requests start this way until some
// router clause sets the session somehow.
var AnonymousSession Session = anonymousSession{nil}

type anonymousSession struct {
	*secret.Secret
}

// Contrary to what you might expect, this actually always returns that the
// session is expired. This is because if it did manage to get itself
// accidentally stored somewhere, it should always be purged. Another one
// can trivially be created when it is necessary.
func (as anonymousSession) Expired() bool {
	return true
}

func (as anonymousSession) Expire() {}

func (as anonymousSession) SessionID() (bool, SessionID) {
	return false, NoSessionID
}

func (as anonymousSession) Identity() *identity.Identity {
	if anonymousIdentity == nil {
		return identity.AnonymousIdentity
	}
	return anonymousIdentity
}

var ErrSessionDoesNotSupportStreams = errors.New("session type does not support streams")

func (as anonymousSession) NewStream() (*strest.Stream, error) {
	return nil, ErrSessionDoesNotSupportStreams
}

func (as anonymousSession) GetStream([]byte) (*strest.Stream, error) {
	return nil, ErrSessionDoesNotSupportStreams
}

func (as anonymousSession) ActiveStreams() []strest.StreamID {
	return nil
}

/*

How Does Authentication Work

1. User hits resource that requires authentication.
2. A new session is created with the Unauthenticated auth.
3. This key is sent via cookie.
4. We store the request in the session, frozen with the remaining
   url.
5. If the user authenticates, we resume the request and send the new
   session.
6. If the user never authenticates, the session terminates.

*/
