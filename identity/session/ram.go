package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/strest"
)

var ss SessionServer = &RAMSessionServer{}

// This file defines a session server that functions entirely in RAM.
//
// Along with being the simplest session to understand (since it has no
// serialization, network, DB, etc. concerns), RAMSessionServer can also be
// a valid session server for real deployment, if you have a situation
// where you don't really mind losing all session info upon process
// restart. It is designed to periodically purge old sessions properly,
// which is also a process that shouldn't occur if this is going to hold
// thousands+ of sessions, so don't use it for that. FIXME: Do that purge.
// Since that purge is done with the lock on sessions held, it will block
// anyone attempting to get a session. Given the speed of modern computers,
// this is pretty trivial into the hundreds and even thousands, but if you
// tried to scale this up would be a problem.

// A RAMSessionServer serves out RAMSessions, holding them in RAM.
//
// While this obviously has scale limits, this is intended to be suitable for
// sufficiently small deployments, such as internal tools where there is a
// clealy-bounded number of people who could even conceivably be logged in,
// or even potentially websites where only a small number of people have
// administrative access, where the vast majority of users are unauth'ed.
type RAMSessionServer struct {
	sessions           map[SessionID]*RAMSession
	sessionIDGenerator *SessionIDGenerator
	secretGenerator    *secret.Generator
	*RAMSessionSettings

	// This locks the session map and all the expiration times on the sessions.
	sync.Mutex
}

type RAMSessionSettings struct {
	Timeout time.Duration
	abtime.AbstractTime
}

// NewRAMServer returns a new RAM-based session server, using the given
// settings. Once the settings have been passed to this object you must
// not modify them.
func NewRAMServer(sig *SessionIDGenerator, secretGenerator *secret.Generator, settings *RAMSessionSettings) *RAMSessionServer {
	ss := &RAMSessionServer{
		sessions:           map[SessionID]*RAMSession{},
		sessionIDGenerator: sig,
		secretGenerator:    secretGenerator,
		RAMSessionSettings: settings,
	}
	if settings.Timeout == 0 {
		settings.Timeout = time.Hour
	}
	if settings.AbstractTime == nil {
		settings.AbstractTime = abtime.NewRealTime()
	}
	return ss
}

func (rss *RAMSessionServer) GetSession(sk SessionID) (Session, error) {
	rss.Lock()
	session := rss.sessions[sk]
	rss.Unlock()

	if session == nil {
		return nil, ErrSessionNotFound
	} else {
		now := rss.Now()
		if session.Expired() {
			delete(rss.sessions, sk)
			return nil, ErrSessionNotFound
		} else {
			session.ExpirationTime = now.Add(rss.Timeout)
			return session, nil
		}
	}
}

func (rss *RAMSessionServer) GetAuthenticationUnwrapper(id string) (secret.AuthenticationUnwrapper, error) {
	return rss.GetSession(SessionID(id))
}

func (rss *RAMSessionServer) NewSession(identity *identity.Identity) (Session, error) {
	now := rss.Now()

	fmt.Println("\n\nMaking new session\n\n")
	spew.Dump(identity)

	session := &RAMSession{
		ExpirationTime: now.Add(rss.Timeout),
		sessionID:      rss.sessionIDGenerator.Get(),
		Secret:         rss.secretGenerator.Get(),
		id:             identity,
		rss:            rss,
		streams:        map[strest.StreamID]*strest.Stream{},
	}
	rss.sessions[session.sessionID] = session

	return session, nil
}

var ErrStreamNotFound = errors.New("stream not found by id")

// A RAMSession is a basic session handed out by a RAMSessionServer.
//
// It may also be useful as a generic session implementation for
// anything keeping them in RAM, or serve as a useful serialization target.
// Stay tuned.
type RAMSession struct {
	ExpirationTime time.Time
	sessionID      SessionID
	id             *identity.Identity
	*secret.Secret

	// this should be stripped from any serializations, and reconstituted
	// from current settings
	rss *RAMSessionServer

	sync.Mutex
	streams map[strest.StreamID]*strest.Stream
}

func (rs *RAMSession) Expired() bool {
	now := rs.rss.Now()
	rs.rss.Lock()
	expiration := rs.ExpirationTime
	rs.rss.Unlock()
	return now.After(expiration)
}

var expired = time.Unix(279835200, 0)

func (rs *RAMSession) Expire() {
	rs.rss.Lock()
	rs.ExpirationTime = expired
	rs.rss.Unlock()
}

func (rs *RAMSession) SessionID() (bool, SessionID) {
	return true, rs.sessionID
}

func (rs *RAMSession) Identity() *identity.Identity {
	return rs.id
}

func thirtytwoRandomBytes(r io.Reader) []byte {
	b := make([]byte, 32, 32)
	_, err := r.Read(b)
	if err != nil {
		panic("Can't get random bytes: " + err.Error())
	}
	return b
}

func (rs *RAMSession) NewStream() (*strest.Stream, error) {
	fmt.Println("Getting new stream from ram session")
	id := strest.StreamID(base64.StdEncoding.EncodeToString(thirtytwoRandomBytes(rand.Reader)))
	stream := strest.NewStream(id)

	rs.Lock()
	rs.streams[id] = stream
	rs.Unlock()

	return stream, nil
}

func (rs *RAMSession) GetStream(signedSid []byte) (*strest.Stream, error) {
	sid, err := rs.UnwrapAuthentication(signedSid)
	if err != nil {
		// return an error indistinguishable from the 'not found' case on
		// purpose, to not leak whether the signature was correct.
		return nil, ErrStreamNotFound
	}
	rs.Lock()
	stream, haveStream := rs.streams[strest.StreamID(string(sid))]
	rs.Unlock()
	if !haveStream {
		return nil, ErrStreamNotFound
	}

	return stream, nil
}

func (rs *RAMSession) ActiveStreams() []strest.StreamID {
	streams := make([]strest.StreamID, 0, len(rs.streams))

	rs.Lock()
	for streamID := range rs.streams {
		streams = append(streams, streamID)
	}
	rs.Unlock()

	return streams
}
