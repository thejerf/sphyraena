package samples

import (
	"crypto/subtle"
	"errors"
	"github.com/thejerf/sphyraena/identity/auth/enticate"
	"github.com/thejerf/sphyraena/unicode"
)

const (
	// This is the maximum size of either the password or the user.
	MaxSize = 128
)

var ErrTooLarge = errors.New("username or password too large")

// HardcodedAuthentication provides a resource that will check the incoming
// username and password form against a hardcoded list, and either serve up
// the given resource as a password form if given, or re-run the routing
// request with new authentication if the authentication works.
//
// This is primarily for A: development and B: showing the simplest
// possible authentication provider.
//
// Both username and password are case-sensitive. Passwords are limited to
// 128 bytes.
type HardcodedAuthentication struct {
	users map[unicode.NFKCNormalized]unicode.NFKCNormalized
}

func NewHardcodedAuth() *HardcodedAuthentication {
	return &HardcodedAuthentication{
		map[unicode.NFKCNormalized]unicode.NFKCNormalized{},
	}
}

// AddUser will add the given username/password combination to the
// hardcoded authenticator. If the given username is already assigned to a
// given password, it will be overwritten.
//
// The only error that can result is that the username or the password is
// too long.
func (ha *HardcodedAuthentication) AddUser(username, password string) error {
	user := unicode.NFKCNormalize(username)
	pw := unicode.NFKCNormalize(password)
	if len(username) > MaxSize {
		return ErrTooLarge
	}
	if len(password) > MaxSize {
		return ErrTooLarge
	}
	ha.users[user] = pw
	return nil
}

func (ha *HardcodedAuthentication) Authenticate(username, password unicode.NFKCNormalized) (enticate.Authentication, enticate.AuthError) {
	if len(username.String()) == 0 || len(password.String()) == 0 {
		return nil, enticate.WrongUserOrPassword()
	}

	correctPassword, haveUser := ha.users[username]

	// guaranteed that the correct password is not > MaxSize by the AddUser
	// method above; can not insert a password that is too long.
	given := make([]byte, MaxSize, MaxSize)
	copy(given, password.String())
	correct := make([]byte, MaxSize, MaxSize)
	copy(correct, correctPassword.String())

	compare := subtle.ConstantTimeCompare(given, correct)

	if compare == 1 && haveUser {
		return &enticate.NamedUser{username}, nil
		// redo request
	} else {
		return nil, enticate.WrongUserOrPassword()
	}
}
