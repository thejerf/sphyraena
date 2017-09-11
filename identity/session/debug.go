package session

import (
	"time"

	"github.com/thejerf/abtime"
	"github.com/thejerf/sphyraena/identity"
	"github.com/thejerf/sphyraena/identity/auth/enticate"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/sphyraena/unicode"
)

// this implements a temporary debugging sessions server that ignores all
// authentication and just always returns the same session. If you are not
// Jeremy Bowers and you are seeing this as anything but a file in git
// history, and probably not even that, Jeremy Bowers has screwed up.

type DebugSessionServer struct {
	session *RAMSession
}

func (dss *DebugSessionServer) GetSession(sID SessionID) (Session, error) {
	return dss.session, nil
}

func (dss *DebugSessionServer) NewSession(id *identity.Identity) (Session, error) {
	return dss.session, nil
}

func (dss *DebugSessionServer) GetAuthenticationUnwrapper(string) (secret.AuthenticationUnwrapper, error) {
	return dss.session, nil
}

func NewDebugSessionServer() *DebugSessionServer {
	ramSession := &RAMSession{
		ExpirationTime: time.Now().Add(time.Hour * 365 * 24),
		sessionID:      SessionID("badSessionID"),
		Secret:         secret.New([]byte("badsecret")),
		id:             &identity.Identity{&enticate.NamedUser{unicode.NFKCNormalize("jbowers")}},
		// dirty dirty dirty, but it's just for temp
		rss: &RAMSessionServer{
			RAMSessionSettings: &RAMSessionSettings{
				Timeout:      time.Hour * 365 * 24,
				AbstractTime: abtime.NewRealTime(),
			},
		},
	}

	return &DebugSessionServer{ramSession}
}
