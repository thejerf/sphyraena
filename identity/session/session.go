package session

import (
	"errors"

	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/strest"
)

// TODO:
// A better account lockout bypass mechanism. If the account gets too many
// login attempts, it gets locked out for a period of time. If the user
// would like, they can send themselves an email which will let them in,
// but only on that session. That way external attackers are still locked
// out, even if they keep hammering.
//
// Some internal discussions observe:
// * Lockout should only be incremented on a "deliberate" login. Passive
//   things like network authentication shouldn't be able to contribute to
//   lockout.
// * A session ought to be able to bypass the lockout for some amount of
//   tries, via doing something like setting a cookie that refreshes the
//   attempt count within the session. This would allow a valid user to
//   continue logging in even during attack.

// A Session represents a handle for interacting with a session.
//
// It may be a session, but it may also be some sort of reference to a way
// of manipulating sessions remotely. It's specified this way so that
// Sphyraena can extend beyond one server.
type Session interface {
	// This returns if the session is expired or not.
	//
	// There are many storage systems that allow you to place timeouts on
	// a value, after which it will disappear. In that case, this can be
	// implemented by seeing whether it has so timed out. If, for instance,
	// you are using the RAMSessionServer, though, a session won't be
	// cleaned up until the Server runs through and checks for expirations,
	// and deletes those that are expired.
	Expired() bool

	// This should terminate the session completely. Given that our
	// "sessions" include streaming, this includes terminating all living
	// streams. Expiring an expired session should not be an error. Expire
	// may be called from any goroutine.
	Expire()

	// Returns whether we have a session ID, and the relevant session ID
	// if there is one. (For instance, the AnonymousSession won't have an ID.)
	SessionID() (bool, SessionID)

	// This returns the Identity currently associated with this
	// session.
	//
	// Security note: An Identity MUST be constant for a given
	// session. See discussion on the SessionCreator interface.
	Identity() *identity.Identity

	// This returns a new stream that can be identified by ID, or an error
	// if no stream can be created (perhaps because this is for some reason
	// an impoverished session that lacks that capability).
	//
	// FIXME: In general, sessions can't provide GetStream. In which case,
	// why are they providing the streams at all? The answer is probably
	// that we need a distinction between sessions, stream masters, and
	// authentication that this mixes in too freely right now.
	NewStream() (*strest.Stream, error)

	// This retrieves a stream by the given key. If it is from this
	// session, the stream will be returned. (FIXME: or created?)
	// This can be problematic with streams that may live in other
	// processes, requiring some sort of forwarding arrangment or a message
	// bus or something.
	GetStream([]byte) (*strest.Stream, error)

	// Returns active streams.
	//
	// Note this is inherently racy; new streams may be created at any
	// moment and you may be unable to retrieve any given stream that is
	// returned. Still, this can be useful information. This may return nil.
	// ActiveStreams() []strest.StreamID

	// The session must contain something that can be used to authenticate
	// and validate that authentication. This is usually done by composing
	// in a secret.Secret value, but you can also forward these interfaces.
	secret.Authenticator
	secret.AuthenticationUnwrapper
}

// FIXME: this should have a slot for the underlying problem

var ErrSessionNotFound = errors.New("session not found")

// A SessionServer takes SessionIDs, and returns Sessions, with or without
// creating them if they do not currently exist.
type SessionServer interface {
	// since sessions may be on the network, in the DB, etc., it can still
	// be an error that we may want to log if we can't get a session.
	// ErrSessionNotFound must be returned to indicate that it wasn't found,
	// but nothing has particularly gone wrong.
	//
	// If a session is expired, it should not be returned; users of your
	// SessionServer should not need to track that.
	GetSession(SessionID) (Session, error)

	NewSession(*identity.Identity) (Session, error)

	secret.AuthenticationUnwrappers
}
