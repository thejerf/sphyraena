package internal

import (
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/secret"
)

// A MarshalFileSession is used to marshal the given file session. This
// should be in internal.
type MarshalFileSession struct {
	SessionID string             `json:"session_id"`
	Identity  *identity.Identity `json:"identity"`
	Secret    *secret.Secret     `json:"secret"`
}
