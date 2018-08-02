package session

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-test/deep"
	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/auth/enticate"
	"github.com/thejerf/sphyraena/secret"
)

func getDiskSession(t *testing.T) (*FilesystemServer, func()) {
	dir, err := ioutil.TempDir("", "sphyraena_session_disk_test")
	if err != nil {
		t.Fatalf("Couldn't get temporary dir: %v", err)
	}

	idGen := NewSessionIDGenerator(0, []byte("0123456789012345"))
	go idGen.Serve()
	secretGen := secret.NewGenerator(8)
	go secretGen.Serve()

	manTime := abtime.NewManual()

	dss := NewFilesystemServer(dir, idGen, secretGen,
		&FilesystemServerSettings{
			AbstractTime: manTime,
			Timeout:      time.Hour,
		})

	return dss, func() {
		os.RemoveAll(dir)
		idGen.Stop()
		secretGen.Stop()
	}
}

func TestDirSessions(t *testing.T) {
	dss, deffunc := getDiskSession(t)
	defer deffunc()
	manTime := dss.AbstractTime.(*abtime.ManualTime)

	id := &identity.Identity{enticate.GetNamedUser("test")}

	// Get a session for our named user tmp
	session, err := dss.NewSession(id)
	if err != nil {
		t.Fatalf("Could not get user session: %v", err)
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
	// the times won't match so just blow them away
	sessionOriginal := session.(*fileSession).lastRefreshTime
	session2Original := session.(*fileSession).lastRefreshTime
	session.(*fileSession).lastRefreshTime = time.Time{}
	session2.(*fileSession).lastRefreshTime = time.Time{}
	diffs := deep.Equal(session, session2)
	if len(diffs) != 0 {
		spew.Dump(diffs)
		t.Fatal("Retrieved session is not identical to the originally-created session")
	}

	// Now let's advance the clock and see things expire.
	session.(*fileSession).lastRefreshTime = sessionOriginal
	session2.(*fileSession).lastRefreshTime = session2Original
	if session.Expired() {
		t.Fatal("Sessions start out expired, huh?")
	}
	if session2.Expired() {
		t.Fatal("Sessions start out expired when loaded from the disk?")
	}

	manTime.Advance(2 * time.Hour)
	if !session.Expired() {
		t.Fatal("Created sessions do not expire after time advances")
	}
	if !session2.Expired() {
		t.Fatal("Loaded sessions do not expire after time advances")
	}

	_, err = dss.GetSession(sessionID)
	if err == nil {
		t.Fatal("Can load expired sessions!")
	}
}

func TestExpiringSession(t *testing.T) {
	dss, deffunc := getDiskSession(t)
	defer deffunc()

	id := &identity.Identity{enticate.GetNamedUser("test")}

	session, err := dss.NewSession(id)
	if err != nil {
		t.Fatalf("Could not get user session: %v", session)
	}

	session.Expire()

	_, sessionID := session.SessionID()
	session, err = dss.GetSession(sessionID)
	if session != nil || err == nil {
		t.Fatal("Can load expired sessions")
	}
}
