/*

Package session manages Sphyraena's session ID generation.

See https://www.owasp.org/index.php/Session_Management_Cheat_Sheet .

Session IDs are based on the cryptographic PRNG that Go backs to. If
you do not sufficiently seed this with something, which can pretty much
only happen bringing up VMs, you may have guessable session IDs at first.
It's your job to ensure that your OS correctly stores entropy between
boots, and that you seed the CPRNG somehow between VM bring-ups.

For testing purposes, to create test SessionIDs, just create whatever
string you like and convert it to a session.SessionID in your code.
While a randomly created session won't be accepted by Sphyraena as
a whole, it will suit many testing scenarios where you are not interacting
with Sphyraena.

*/
package session

// FIXME: We're going to eliminate the generator server, which will remove
// the duplication present here.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
)

const (
	// this is a result, not a cause; session keys generated and signed by
	// a SessionIDGenerator should be of length 44.
	// this is just used to make the checks read nicely
	sessionIDLength = 88
)

// Going from the cheat sheet, Session ID properties:
//
// * ID Name Fingerprinting: Well, due to the cookie signing of the name
//   itself, it's probably going to be unmistakeably Sphryaena. But
//   I'm not willing to give up the signing...
// * Session ID Length: Minimum suggested is 128 bits, this has a
//   cryptographic RNG generating 256 bits.
// * Session ID entropy: Inherited from your CPRNG implementation, but this
//   should in all cases be as good as Sphyraena can possibly do. (There's
//   nothing Sphyraena can do about low-entropy OS pools.)
// * Session ID content: A cryptographically-secure random number.
//   I don't truck with seeing what can be snuck into the session;
//   Sphyraena is sufficiently concerned about security that the only
//   answer we're going to accept is dedicating the storage to correctly
//   record sessions.
// 5.1: This is a "strict" session; an HMAC check must pass that proves we
//   generated this session ID.
// Cookies are used by default, marked both "secure" and "httponly".

// A SessionID is what is emitted to the user in the form of a cookie or
// some other token they return to us.
type SessionID string

// NoSessionID can be returned by an implementation of a SessionID method
// if it is going to return false. This will always be of the correct type,
// even if that type changes in future versions.
var NoSessionID = SessionID("")

// This is a secret key contained inside of a session. It should NEVER go
// to the user!
type SecretKey []byte

// Session keys are moderately expensive to generate. We buffer them up
// using Go channels so that when we need a new one, we should ideally have
// one available. Worst case scenario we have to generate it on the spot.
type SessionIDGenerator struct {
	output     chan SessionID
	stop       chan struct{}
	hmacKey    []byte
	hmacer     hash.Hash
	randReader io.Reader
}

// NewSessionIDGenerator returns a SessionIDGenerator that will buffer up
// to the given number of keys in advance.
//
// A bufferSize of 0 will use the framework's default, which is currently
// 128 but is not promised to stay constant in the future.
//
// key is used to validate sessions. If you are running multiple instances
// of Sphyraena and users are not bound to specific Sphyraena instances,
// or if your sessions should persist beyond a restart of Sphyraena, this
// key should be the same between restarts and between servers. However, it
// should otherwise be secret. If this is nil, 32 bytes will be pulled from
// the system CSPRNG. If you're generating your own key, 32 bytes pulled
// from your CSPRNG is a good place to start.
//
// This key should be treated as a secret. If an attacker obtains it, it
// does not mean they can read other people's sessions without further
// guessing the session ID, but it does mean they can generate arbitrary
// valid session IDs on their own, which Sphyraena will then willingly
// create.
//
// Changing this key invalidates all current sessions.
func NewSessionIDGenerator(bufferSize int, key []byte) *SessionIDGenerator {
	// on Linux, we can check the entropy in
	// /proc/sys/kernel/random/entropy_avail and complain if it's not big enough.
	if bufferSize == 0 {
		bufferSize = 128
	}

	if len(key) == 0 {
		key = make([]byte, 32)
		rand.Read(key)
	}

	return &SessionIDGenerator{
		make(chan SessionID, bufferSize),
		make(chan struct{}),
		key,
		hmac.New(sha256.New, []byte(key)),
		rand.Reader,
	}
}

func (skg *SessionIDGenerator) Serve() {
	// this code plays a bit of silly buggers with slices, so follow along
	// with me:
	// first: we make a 64-byte length and cap slice...
	sessionID := make([]byte, 64)
	for {
		// second: cut it down to 32-bytes (continue in generate...)
		sessionID = sessionID[:32]
		select {
		case skg.output <- skg.generate(sessionID):
		case <-skg.stop:
			return
		}
	}
}

func (skg *SessionIDGenerator) Stop() {
	// stopped here is implicitly shielded by the closing of the channel.
	skg.stop <- struct{}{}
}

// Get retrieves a fresh new SessionID.
func (skg *SessionIDGenerator) Get() SessionID {
	return <-skg.output
}

// separated for easy testing; conceptually this is just inline in Serve.
func (skg *SessionIDGenerator) generate(sessionID []byte) SessionID {
	// third: ... read 32 bytes into our 32-byte length slice
	n, err := skg.randReader.Read(sessionID)
	if err != nil {
		panic(fmt.Errorf("While making session keys, couldn't read from PSRNG: %s", err.Error()))
	}
	if n != 32 {
		panic(fmt.Errorf("While making session keys, could only read %d bytes", n))
	}
	// per the interface hash.Hash, this can not return an error
	_, _ = skg.hmacer.Write(sessionID)
	// fourth: append the 32-bytes of hmac onto the sessionID, which will
	// then return the now 64-byte-len slice, which did not have to be
	// resized because that's where it started.
	// We can't help allocating for these but at least we should avoid
	// allocating too many things.
	// This is a bit of a brute-force way of *ensuring* that the only valid
	// session IDs come from us, meaning that user implementations of
	// SessionServers do not need to worry about session fixation attacks
	// where an attacker picks a session; an attacker is not capable of
	// correctly naming a new session ID Sphyraena will accept. Combined with
	// the session cookie not being available to Javascript, only accepted
	// over HTTPS, etc., it should prevent session fixation entirely.
	sessionID = skg.hmacer.Sum(sessionID)
	skg.hmacer.Reset()
	// finally:  we Base64 that into a string, which is now independent of the
	// slice and we can re-use the same 64-byte slice again and again.
	return SessionID(base64.StdEncoding.EncodeToString(sessionID))
}

// Check validates that a given session key is a session key validly
// generated by a SessionIDGenerator with the same key as this generator.
func (skg *SessionIDGenerator) Check(sessionID SessionID) bool {
	if len(sessionID) != sessionIDLength {
		return false
	}

	b, err := base64.StdEncoding.DecodeString(string(sessionID))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, skg.hmacKey)
	mac.Write(b[:32])
	expected := mac.Sum(nil)
	return hmac.Equal(expected, b[32:])
}

type SessionIDManager interface {
	Get() SessionID
	Check(sessionID SessionID) bool
}

type SessionIDs struct {
	secret []byte
}

func (sids SessionIDs) Get() SessionID {
	sessionID := make([]byte, 64)

	n, err := rand.Reader.Read(sessionID[:32])
	if err != nil {
		panic(fmt.Errorf("While making session keys, couldn't read from PSRNG: %s", err.Error()))
	}
	if n != 32 {
		panic(fmt.Errorf("While making session keys, could only read %d bytes", n))
	}
	// per the interface hash.Hash, this can not return an error
	hmacer := hmac.New(sha256.New, sids.secret)
	_, _ = hmacer.Write(sessionID)
	// fourth: append the 32-bytes of hmac onto the sessionID, which will
	// then return the now 64-byte-len slice, which did not have to be
	// resized because that's where it started.
	// We can't help allocating for these but at least we should avoid
	// allocating too many things.
	// This is a bit of a brute-force way of *ensuring* that the only valid
	// session IDs come from us, meaning that user implementations of
	// SessionServers do not need to worry about session fixation attacks
	// where an attacker picks a session; an attacker is not capable of
	// correctly naming a new session ID Sphyraena will accept. Combined with
	// the session cookie not being available to Javascript, only accepted
	// over HTTPS, etc., it should prevent session fixation entirely.
	sessionID = hmacer.Sum(sessionID)
	// finally:  we Base64 that into a string, which is now independent of the
	// slice and we can re-use the same 64-byte slice again and again.
	return SessionID(base64.StdEncoding.EncodeToString(sessionID))
}

func (sids SessionIDs) Check(sessionID SessionID) bool {
	if len(sessionID) != sessionIDLength {
		return false
	}

	b, err := base64.StdEncoding.DecodeString(string(sessionID))
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, sids.secret)
	mac.Write(b[:32])
	expected := mac.Sum(nil)
	return hmac.Equal(expected, b[32:])
}

func NewSessionIDs(secret []byte) SessionIDs {
	return SessionIDs{secret}
}
