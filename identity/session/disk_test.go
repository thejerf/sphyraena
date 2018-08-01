package session

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-test/deep"
	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/auth/enticate"
	"github.com/thejerf/sphyraena/secret"
)

func TestDirSessions(t *testing.T) {
	dir, err := ioutil.TempDir("", "sphyraena_session_disk_test")
	if err != nil {
		t.Fatalf("Couldn't get temporary dir: %v", err)
	}
	defer os.RemoveAll(dir)

	idGen := NewSessionIDGenerator(0, []byte("0123456789012345"))
	go idGen.Serve()
	defer idGen.Stop()
	secretGen := secret.NewGenerator(8)
	go secretGen.Serve()
	defer secretGen.Stop()

	manTime := abtime.NewManual()

	dss := NewDiskServer(dir, idGen, secretGen,
		&DirSessionSettings{AbstractTime: manTime})

	id := &identity.Identity{enticate.GetNamedUser("test")}

	// Get a session for our named user tmp
	session, err := dss.NewSession(id)
	if err != nil {
		t.Fatalf("Could not get user session: %v", session)
	}

	// Fetch the session for the named user
	haveSession, sessionID := session.SessionID()
	if !haveSession {
		t.Fatal("disk sessions do not have an ID?")
	}
	session2, err := dss.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Couldn't get an existing session: %v", err)
	}
	if session.(*fileSession).expirationTime.Equal(session2.(*fileSession).expirationTime) {
		// FIXME: Figure out if this is a good thing or not. I'm tempted to
		// make a UTCTime type to guarantee UTC-ness.
		session2.(*fileSession).expirationTime = session.(*fileSession).expirationTime
	}
	diffs := deep.Equal(session, session2)
	if len(diffs) != 0 {
		spew.Dump(diffs)
		t.Fatal("Retrieved session is not identical to the originally-created session")
	}
}
