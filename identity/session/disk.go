package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/strest"
)

// This file defines a session server that functions on disk.
//
// This is the second simplest session server to understand, as while it is
// very simple it does have serialization and disk concerns.
//
// It is viable as a real session server, as long as you're not going to
// have too many users for one directory to store. You end up with one file
// per user in the directory, and at the moment, nothing clears out expired
// files.

// A DirSessionServer serves out FileSessions.
//
// While this obviously has scale limits, this is intended to be suitable
// for sufficiently small deployments, such as internal tools. You will end
// up with one small file on the disk per user who can be logged in.
type DirSessionServer struct {
	directory          string
	sessionIDGenerator *SessionIDGenerator
	secretGenerator    *secret.Generator
	*DirSessionSettings

	// synchronize all access through this, to simplify things.
	lock sync.Mutex
}

type DirSessionSettings struct {
	Timeout time.Duration
	abtime.AbstractTime
}

// A MarshalFileSession is used to marshal the given file session. This
// should be in internal.
type MarshalFileSession struct {
	ExpirationTime time.Time          `json:"expiration_time"`
	SessionID      SessionID          `json:"session_id"`
	Identity       *identity.Identity `json:"identity"`

	Secret *secret.Secret `json:"secret"`
}

type fileSession struct {
	expirationTime time.Time
	sessionID      SessionID
	identity       *identity.Identity
	*secret.Secret

	dss *DirSessionServer
}

// FIXME: add some stuff to walk through sessions and expire them off the disk

// NewDirSessionServer returns a new disk-based session server, using the
// given settings. Once the settings have been passed to this object you
// must not modify them.
func NewDiskServer(
	directory string,
	sig *SessionIDGenerator,
	secretGenerator *secret.Generator,
	settings *DirSessionSettings,
) *DirSessionServer {
	if settings == nil {
		settings = &DirSessionSettings{}
	}
	ds := &DirSessionServer{
		directory:          directory,
		sessionIDGenerator: sig,
		secretGenerator:    secretGenerator,
		DirSessionSettings: settings,
	}
	if settings.Timeout == 0 {
		settings.Timeout = time.Hour
	}
	if settings.AbstractTime == nil {
		settings.AbstractTime = abtime.NewRealTime()
	}
	return ds
}

// this is a slightly paranoid file name replacer, to ensure that our
// session ID can be safely stored on disk. Trying to be multi-OS compliant
// but that's hard to test.
var encoder = strings.NewReplacer(
	"/", "!1",
	"\\", "!2",
	"?", "!3",
	"*", "!4",
	":", "!5",
	"\"", "!6",
	"<", "!7",
	">", "!8",
	"!", "!9",
)

func (dss *DirSessionServer) sessionToFile(sID SessionID) string {
	return filepath.Join(dss.directory, encoder.Replace(string(sID)))
}

func (dss *DirSessionServer) GetSession(sID SessionID) (Session, error) {
	filename := dss.sessionToFile(sID)

	var f io.Reader
	f, err := os.Open(filename)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	f = io.TeeReader(f, os.Stderr)

	fs := &MarshalFileSession{
		Identity: &identity.Identity{},
	}

	decoder := json.NewDecoder(f)
	err = decoder.Decode(fs)
	if err != nil {
		return nil, err
	}

	if fs.ExpirationTime.IsZero() {
		return nil, errors.New("file session:  missing expiration")
	}
	if fs.SessionID == SessionID("") {
		return nil, errors.New("file session: session ID missing")
	}
	if fs.SessionID != sID {
		// The only way I can think of for this to happen is case mismatch
		// on a case-insensitive file system. I don't think it's very
		// likely. But if it did happen, it would mean that this user is
		// going to get some random other identity, which is catastrophic,
		// so I'd rather scream and die and have this session just be
		// mysteriously invalid than have the wrong authentication.
		return nil, errors.New("file session: session ID mismatch")
	}
	if fs.Identity.Authentication == nil {
		return nil, errors.New("file session: authentication missing")
	}
	if fs.Secret.IsZero() {
		return nil, errors.New("file session: missing secret")
	}

	// FIXME: Need to add time to the expiration and record that.
	// FIXME: Get excessively clever and use the metadata of the file
	// itself for expirations, so the process of updating the file is
	// simply touching it, allowing us to avoid race conditions on the file
	// entirely?

	return &fileSession{
		fs.ExpirationTime,
		fs.SessionID,
		fs.Identity,
		fs.Secret,
		dss,
	}, nil
}

func (dss *DirSessionServer) NewSession(id *identity.Identity) (Session, error) {
	now := dss.Now()

	fs := &MarshalFileSession{
		ExpirationTime: now.Add(dss.Timeout),
		SessionID:      dss.sessionIDGenerator.Get(),
		Secret:         dss.secretGenerator.Get(),
		Identity:       id,
	}
	filename := dss.sessionToFile(fs.SessionID)

	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	encoder := json.NewEncoder(f)
	err = encoder.Encode(fs)
	if err != nil {
		return nil, err
	}
	_ = f.Close()

	return &fileSession{
		fs.ExpirationTime,
		fs.SessionID,
		fs.Identity,
		fs.Secret,
		dss,
	}, nil
}

func (dss *DirSessionServer) GetAuthenticationUnwrapper(id string) (secret.AuthenticationUnwrapper, error) {
	return dss.GetSession(SessionID(id))
}

func (fs *fileSession) Expired() bool {
	now := fs.dss.Now()

	return now.After(fs.expirationTime)
}

func (fs *fileSession) Expire() {
	os.Remove(fs.dss.sessionToFile(fs.sessionID))
	// FIXME: Need to be able to set logging to catch this here
	now := fs.dss.Now()
	fs.expirationTime = now.Add(-time.Second)
}

func (fs *fileSession) SessionID() (bool, SessionID) {
	return true, fs.sessionID
}

func (fs *fileSession) Identity() *identity.Identity {
	return fs.identity
}

func (fs *fileSession) NewStream() (*strest.Stream, error) {
	id := strest.StreamID(base64.StdEncoding.EncodeToString(thirtytwoRandomBytes(rand.Reader)))
	stream := strest.NewStream(id)

	return stream, nil
}
