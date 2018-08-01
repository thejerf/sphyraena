package enticate

import (
	"errors"
	"strings"
)

const (
	EnticateSeparator = "⁝"
)

var ErrUnknownAuthType = errors.New("Unknown authentication type")

var ErrNilAuth = errors.New("Nil authentication passed to Marshal")

var authentications = map[string]Authentication{}

// Register will register the given Authenication with sphyraena, so it can
// be Unmarshaled into.
//
// All Registrations must be completed before any Unmarshal or Marshal
// calls are made.
//
// ⁝ is a reserved character for authentication names. Due to the expected
// rarity of this coming up, this method will panic if that constraint is
// violated.
func Register(auth Authentication) {
	name := auth.AuthenticationName()
	if strings.Contains(name, EnticateSeparator) {
		panic("authentication name can't contain tricolon (" +
			EnticateSeparator + ")")
	}
	authentications[name] = auth
}

// Unmarshal takes the tuple of the authentication's name and the
// authentication info, and turns it into an Authentication that can be
// used by the rest of Sphyraena.
//
// The authName should match the return value of the AuthenticationName()
// of the relevant Authentication type, and it must have been Register()ed
// before this method is called or you will get ErrUnknownAuthType.
func Unmarshal(authName string, authInfo []byte) (Authentication, error) {
	authType := authentications[authName]
	if authType == nil {
		return nil, ErrUnknownAuthType
	}

	empty := authType.Empty()

	err := empty.UnmarshalText(authInfo)
	if err != nil {
		return nil, err
	}
	return empty, nil
}

// Marshal takes a given Authentication, and returns the authName and
// authInfo for the given Authentication. There is only an error if the
// Authentication returns an error from its MarshalText method or if the
// passed-in auth is nil.
func Marshal(auth Authentication) (string, []byte, error) {
	if auth == nil {
		return "", nil, ErrNilAuth
	}

	txt, err := auth.MarshalText()
	if err != nil {
		return "", nil, ErrNilAuth
	}
	name := auth.AuthenticationName()
	return name, txt, nil
}
