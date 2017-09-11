package enticate

import (
	"errors"

	"github.com/thejerf/sphyraena/unicode"
)

// An AuthError is given when an authentication could not be successfully
// completed. In addition to whatever custom stuff you may like, this
// defines a default suite of reasons why the authentication may have
// failed. This is intended to give symbolic reasons for failure that may
// be used for return errors in preference to the underlying error's string
// value, which is intrinsically a single language.
//
// All AuthErrors are also errors.
type AuthError interface {
	error

	// Returns true if this is the wrong user or password.
	WrongUserOrPassword() bool

	// Returns true if this is because the auth service is down.
	AuthServiceDown() bool

	// Returns true if the user may try again, false if they are locked out
	// for some period of time.
	MayTryAgain() bool

	// This error was the result of no authorization being given
	NoAuthGiven() bool
}

type autherror struct {
	error
	wrongUserOrPassword bool
	authServiceDown     bool
	mayTryAgain         bool
	noAuthGiven         bool
}

func (ae *autherror) WrongUserOrPassword() bool {
	return ae.wrongUserOrPassword
}

func (ae *autherror) AuthServiceDown() bool {
	return ae.authServiceDown
}

func (ae *autherror) MayTryAgain() bool {
	return ae.mayTryAgain
}

func (ae *autherror) NoAuthGiven() bool {
	return ae.noAuthGiven
}

// WrongUserOrPassword returns an AuthError which indicates the user had
// the wrong user or password. The user may try again.
func WrongUserOrPassword() AuthError {
	return &autherror{
		error:               errors.New("wrong user or password"),
		wrongUserOrPassword: true,
		mayTryAgain:         true,
	}
}

// AuthServiceDown returns an AuthError which indicates the auth service is
// down. The user may try again, but text renderings should indicate the
// likelihood of repeated failure.
func AuthServiceDown() AuthError {
	return &autherror{
		error:           errors.New("auth service is down"),
		authServiceDown: true,
		mayTryAgain:     true,
	}
}

// LockedOut returns an AuthError which indicates the user has somehow
// become locked out due to excessive login tries.
func LockedOut() AuthError {
	return &autherror{
		error: errors.New("user locked out"),
	}
}

func NoAuthGiven() AuthError {
	return &autherror{
		error:       errors.New("no authorization information given"),
		noAuthGiven: true,
		mayTryAgain: true,
	}
}

type PasswordAuthenticator interface {
	Authenticate(username, password unicode.NFKCNormalized) (Authentication, AuthError)
}
