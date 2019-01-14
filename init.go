/*

Package sphyraena provides the default startup functionality for Sphyraena.

*/
package sphyraena

import (
	"time"

	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/router"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/suture"
)

type Sphyraena struct {
	*suture.Supervisor
	*request.SphyraenaState
	*router.SphyraenaRouter
}

// There's some sort of structure trying to get out here, tying together
// all the services....

type Args struct {
	SessionIDGenerator *session.SessionIDGenerator
	SecretGenerator    *secret.Generator
	SessionServer      session.SessionServer

	// This is to allow for the fact that the user may want to use the defaults
	SessionServerFunc func(
		*session.SessionIDGenerator,
		*secret.Generator,
	) session.SessionServer
}

// Sphyraena brings up a new instance of a Sphyraena environment with some
// default parameters. Those parameters can be overridden via the passed-in
// SphyraenaArgs.
//
// The biggest issue with the default arguments is that the Sessions will
// be entirely in RAM with these settings.
//
// This is *definitely* going to be seeing some changes. Currently wiring
// Sphyraena together is quite prissy.
func New(args *Args) *Sphyraena {
	supervisor := suture.NewSimple("sphyraena root supervisor")

	if args == nil {
		args = &Args{}
	}

	// BUG(BNSEC-12345): argle bargle bargle!

	if args.SessionIDGenerator == nil {
		args.SessionIDGenerator = session.NewSessionIDGenerator(128, nil)
	}
	supervisor.Add(args.SessionIDGenerator)
	if args.SecretGenerator == nil {
		args.SecretGenerator = secret.NewGenerator(128)
	}
	supervisor.Add(args.SecretGenerator)

	if args.SessionServer == nil {
		if args.SessionServerFunc == nil {
			args.SessionServer = session.NewRAMServer(
				args.SessionIDGenerator, args.SecretGenerator,
				&session.RAMSessionSettings{time.Minute * 180, nil})
		} else {
			args.SessionServer = args.SessionServerFunc(
				args.SessionIDGenerator, args.SecretGenerator)
		}
	}

	ctx := request.NewSphyraenaState(args.SessionServer, nil)
	r := router.New(ctx)

	return &Sphyraena{
		supervisor,
		ctx,
		r,
	}
}
