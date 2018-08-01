package secret

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

var SignSuffix = []byte("__!sauthed!_")

// ErrNotAuthenticated means that this authenticator has a secret key, but
// the value passed in is not correctly signed by this key.
var ErrNotAuthenticated = errors.New("this value is not authenticated by this secret")

// ErrNoSecretKey means this represents an authenticator that does not have
// a key to authenticate with.
var ErrNoSecretKey = errors.New("session does not have a secret key")

// base64 encoding is nice, but the standard RFC encodings both use /,
// which is not allowed in cookie names according to a strict reading of
// the RFC even if browsers will probably permit it.
var SignatureEncoding = base64.NewEncoding("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ$%").WithPadding(base64.NoPadding)

// An Authenticator takes in a series of []bytes, and yields the last
// []byte concatenated by the signature for the object.
//
// This initial []bytes contribute to the signature but do not appear in
// the signed value. This allows for things like signing a key/value
// such that the value can't be moved between keys.
//
// AuthenticatingSecret objects can be composed in to conform to this
// interface.
type Authenticator interface {
	Authenticate(...[]byte) ([]byte, error)
}

// AuthenticationUnwrappers is some way we can obtain
// AuthenticationUnwrappers (basically, AuthenticatingSecret objects) by a
// string identifier.
//
// In normal usage this would be a SessionID, but making this a string
// instead of a SessionID means this interface is independent of all types
// in this module.
type AuthenticationUnwrappers interface {
	GetAuthenticationUnwrapper(string) (AuthenticationUnwrapper, error)
}

// In other words, expanding on the previous paragraph, it's normal to copy
// & paste these three interfaces into other code so you can get these
// behaviors without binding to session.

// An AuthenticationUnwrapper is something capable of taking the given
// series of bytes, and verifying whether or not it was sourced from the
// object behind the AuthenticationUnwrapper interface.
//
// This unwraps authentication as given by a corresponding Authenticator
// type, and in order to correctly authenticate, you must pass in all the
// []byte values that were used to authenticate. The signature is expected
// to be in the last one.
//
// This is generally either an AuthenticatingSecret or something composing
// one in.
type AuthenticationUnwrapper interface {
	UnwrapAuthentication(...[]byte) ([]byte, error)
}

// A Secret is an object that can cryptographically sign
// sequences of bytes as having come from something in possession of
// this secret.
//
// Nothing other than serialization code should ever be reaching in to this
// object. Leaking the secret to the user would be a bad thing.
//
// This conforms to the Authenticator and AuthenticationUnwrapper
// interfaces. It is normal to compose these in to structs that need to
// provide this service.
type Secret struct {
	secret []byte
}

func New(secret []byte) *Secret {
	return &Secret{secret}
}

var sigLength = SignatureEncoding.EncodedLen(32)
var fullLength = sigLength + len(SignSuffix)

// The Authenticate method's core functionality is to take a byte slice and
// return a byte slice that is authenticated as having come from this
// Secret.
//
// Multiple byte slices can be passed to Authenticate. Only the last one
// will be authenticated and returned. The additional preceding byte slices
// are still added in to the authentication, though, and the resulting
// value will not authenticate without those values also passed to
// UnwrapAuthentication.
//
// This should be used whenever you wish not just the value itself
// authenticated, but the context of its usage. In particular, when
// authenticating a value being used in any sort of key/value structure it
// is a good idea to include the key as part of the authentication, to
// prevent an attacker from constructing a legal value from one key and
// then substituting it in another key's value.
func (s *Secret) Authenticate(b ...[]byte) ([]byte, error) {
	if s == nil {
		return nil, ErrNoSecretKey
	}
	if len(s.secret) == 0 {
		return nil, ErrNoSecretKey
	}

	sig := s.authenticate(b...)

	result := make([]byte, 0, len(b[len(b)-1])+sigLength)
	result = append(result, b[len(b)-1]...)
	result = append(result, SignSuffix...)
	result = append(result, sig...)
	return result, nil
}

// EscapedWrite takes a writer and a byte slice, and writes to the given
// write the given slice of bytes, with the following twist: All zero bytes
// will be written twice.
//
// This is used by the *Secret to ensure that there's no way to concatenate
// two components of an authenticated set of byte slices in such a way that
// there is ambiguity between, say, ("ab", "c", "d") and ("a", "bc", "d").
//
// Authenticators then should write a single null between fields.
//
// FIXME: Is there actually any reason to leave this public?
func EscapedWrite(w io.Writer, b []byte) (int, error) {
	count := len(b)

	if count == 0 {
		return 0, nil
	}

	start := 0
	end := 0
	total := 0

	for end < count {
		if b[end] == 0 {
			n, err := w.Write(b[start : end+1])
			total += n
			if err != nil {
				return total, err
			}
			start = end
		}
		end++
	}
	n, err := w.Write(b[start:end])
	total += n
	return total, err
}

// WHERE I AM: Something's going wrong here with the signatures. The
// unwrapping isn't going right or something.

// UnwrapAuthentication takes the result of an Authenticate call, and
// either unwraps it and returns the original value if the value is
// authenticated by this secret, or returns nil and ErrNotAuthenticated
// if it is not.
//
// Only the last, signed value will be returned, but in order to validate
// the signature, all values passed in to Authenticate must also be passed
// to this.
func (s *Secret) UnwrapAuthentication(b ...[]byte) ([]byte, error) {
	if s == nil {
		return nil, ErrNoSecretKey
	}
	if len(s.secret) == 0 {
		return nil, ErrNoSecretKey
	}

	last := b[len(b)-1]

	if len(last) < fullLength {
		return nil, ErrNotAuthenticated
	}

	var original [][]byte
	original = append(original, b[:len(b)-1]...)
	original = append(original, last[:len(last)-fullLength])
	expected := s.authenticate(original...)

	if hmac.Equal(expected, last[len(last)-sigLength:]) {
		return last[:len(last)-fullLength], nil
	}
	return nil, ErrNotAuthenticated
}

// This returns just the encoded signature, without the prefix.
func (s *Secret) authenticate(b ...[]byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	for _, bytes := range b {
		_, _ = EscapedWrite(mac, bytes)
		_, _ = mac.Write([]byte{0, 1})
	}
	signature := mac.Sum(nil)

	sig := make([]byte, sigLength)
	SignatureEncoding.Encode(sig, signature)
	return sig
}

// stupidSimpleObfuscate is intended to provide the bare minimum
// obfuscation just to prevent serialized secrets from *literally* being
// the secret. It is not intended to stop dedicated attackers. Should
// Sphyraena become large enough that this becomes a "well known"
// technique... I'm going to declare TOTAL VICTORY because Sphyraena got
// that popular! And still not feel bad.
func stupidSimpleObfuscate(b []byte) []byte {
	b2 := append(make([]byte, 0), b...)
	for idx := range b2 {
		b2[idx] = b2[idx] ^ 85
	}
	return b2
}

func (s *Secret) MarshalBinary() (data []byte, err error) {
	return stupidSimpleObfuscate(s.secret), nil
}

func (s *Secret) UnmarshalBinary(in []byte) error {
	s.secret = stupidSimpleObfuscate(in)
	return nil
}

func (s *Secret) MarshalText() ([]byte, error) {
	encoded := hex.EncodeToString(stupidSimpleObfuscate(s.secret))
	return []byte(encoded), nil
}

func (s *Secret) UnmarshalText(in []byte) error {
	decoded, err := hex.DecodeString(string(in))
	if err != nil {
		return err
	}
	s.secret = stupidSimpleObfuscate([]byte(decoded))
	return nil
}

func (s *Secret) IsZero() bool {
	return len(s.secret) == 0
}
