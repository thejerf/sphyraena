package session

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/auth/enticate"
)

func TestReflectionCode(t *testing.T) {
	fss, deffunc := getDiskSession(t)
	defer deffunc()

	id := &identity.Identity{enticate.GetNamedUser("test")}

	session := &fileSession{}
	err := NewSessionFor(fss, id, session)
	if err != nil {
		t.Fatal("Couldn't get a new session via NewSessionFor")
	}
	_, sessionID := session.SessionID()

	// test getting sessions out via GetSessionFrom
	targetSession := &fileSession{}
	err = GetSessionFrom(fss, sessionID, targetSession)
	if err != nil {
		spew.Dump(err)
		t.Fatal("Couldn't get session via GetSessionFrom")
	}
	if targetSession.Identity().Authentication.LogName() != session.Identity().Authentication.LogName() {
		t.Fatal("Session doesn't match from GetSessionFrom")
	}

	wrongTargetSession := &RAMSession{}
	err = GetSessionFrom(fss, sessionID, wrongTargetSession)
	if err == nil {
		t.Fatal("got the wrong session type from GetSessionFrom")
	}
	err.Error() // coverage test

	err = NewSessionFor(fss, id, wrongTargetSession)
	if err == nil {
		t.Fatal("got the wrong session type from NewSessionFor")
	}
}
