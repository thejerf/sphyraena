package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/session/internal"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/strest"
)

const (
	filesystemServerTicker = 1
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

// A FilesystemServer serves out FileSessions.
//
// While this obviously has scale limits, this is intended to be suitable
// for sufficiently small deployments, such as internal tools. You will end
// up with one small file on the disk per user who can be logged in.
//
// A FilesystemServer is also a suture.Service. The service will scan the
// directory every hour and remove any expired files. It expects that these
// are sessions. Should you stick anything else in that directory, expect
// it to be purged.
//
// It is safe to not use the service, but the files will stack up.
type FilesystemServer struct {
	directory          string
	sessionIDGenerator *SessionIDGenerator
	secretGenerator    *secret.Generator
	*FilesystemServerSettings

	stopScanExpired chan struct{}
	sync            chan struct{} // used in testing
}

type FilesystemServerSettings struct {
	Timeout time.Duration
	abtime.AbstractTime
}

type fileSession struct {
	lastRefreshTime time.Time
	sessionID       SessionID
	identity        *identity.Identity
	*secret.Secret

	fss *FilesystemServer
}

// FIXME: add some stuff to walk through sessions and expire them off the disk

// NewDirSessionServer returns a new disk-based session server, using the
// given settings. Once the settings have been passed to this object you
// must not modify them. The sig and secretGenerator arguments must not be
// nil or this will panic.
func NewFilesystemServer(
	directory string,
	sig *SessionIDGenerator,
	secretGenerator *secret.Generator,
	settings *FilesystemServerSettings) *FilesystemServer {
	if settings == nil {
		settings = &FilesystemServerSettings{}
	}

	if sig == nil {
		panic("SessionIDGenerator required")
	}
	if secretGenerator == nil {
		panic("secret.Generator required")
	}

	ds := &FilesystemServer{
		directory:                filepath.Clean(directory),
		sessionIDGenerator:       sig,
		secretGenerator:          secretGenerator,
		FilesystemServerSettings: settings,
		stopScanExpired:          make(chan struct{}),
		sync:                     make(chan struct{}),
	}
	if settings.Timeout == 0 {
		settings.Timeout = time.Hour
	}
	if settings.AbstractTime == nil {
		settings.AbstractTime = abtime.NewRealTime()
	}
	return ds
}

// Serve implements the suture.Service interface, and scans the directory
// every hour for expired files.
func (fss *FilesystemServer) Serve() {
	everyHour := fss.NewTicker(time.Hour, filesystemServerTicker)

	for {
		select {
		case _, _ = <-fss.stopScanExpired:
			return

		case <-everyHour.Channel():
			now := fss.Now()

			// FIXME: logging
			_ = filepath.Walk(fss.directory, func(
				path string,
				info os.FileInfo,
				err error,
			) error {
				if path == fss.directory {
					// don't remove the root dir, of course
					return nil
				}

				if info.ModTime().Add(fss.Timeout).Before(now) {
					// FIXME: log
					_ = os.Remove(path)
				}

				return nil
			})

		case <-fss.sync:
			// do nothing on purpose; this synchronizes so we can test
		}
	}
}

func (fss *FilesystemServer) Stop() {
	close(fss.stopScanExpired)
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

func (fss *FilesystemServer) sessionToFile(sID string) string {
	return filepath.Join(fss.directory, encoder.Replace(sID))
}

func (fss *FilesystemServer) GetSession(sID SessionID) (Session, error) {
	filename := fss.sessionToFile(string(sID))

	f, err := os.Open(filename)
	if err != nil {
		return nil, ErrSessionNotFound
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	lastRefreshTime := stat.ModTime().UTC()
	now := fss.Now()
	if lastRefreshTime.Add(fss.Timeout).Before(now) {
		return nil, ErrSessionNotFound
	}

	fs := &internal.MarshalFileSession{
		Identity: &identity.Identity{},
	}

	decoder := json.NewDecoder(f)
	err = decoder.Decode(fs)
	if err != nil {
		return nil, err
	}

	if fs.SessionID == "" {
		return nil, errors.New("file session: session ID missing")
	}
	if fs.SessionID != string(sID) {
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

	return &fileSession{
		lastRefreshTime,
		SessionID(fs.SessionID),
		fs.Identity,
		fs.Secret,
		fss,
	}, nil
}

func (fss *FilesystemServer) NewSession(id *identity.Identity) (Session, error) {
	fs := &internal.MarshalFileSession{
		SessionID: string(fss.sessionIDGenerator.Get()),
		Secret:    fss.secretGenerator.Get(),
		Identity:  id,
	}
	filename := fss.sessionToFile(fs.SessionID)

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
		fss.AbstractTime.Now().UTC(),
		SessionID(fs.SessionID),
		fs.Identity,
		fs.Secret,
		fss,
	}, nil
}

func (fss *FilesystemServer) GetAuthenticationUnwrapper(id string) (secret.AuthenticationUnwrapper, error) {
	return fss.GetSession(SessionID(id))
}

func (fs *fileSession) Expired() bool {
	now := fs.fss.Now()

	return fs.lastRefreshTime.Add(fs.fss.Timeout).Before(now)
}

func (fs *fileSession) Expire() {
	os.Remove(fs.fss.sessionToFile(string(fs.sessionID)))
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
