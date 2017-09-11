/*

Package sphyraena provides the default startup functionality for Sphyraena.

*/
package sphyraena

import (
	"time"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/router"
	"github.com/thejerf/sphyraena/secret"
	"github.com/thejerf/suture"
)

type Sphyraena struct {
	*suture.Supervisor
	*context.SphyraenaState
	*router.SphyraenaRouter
}

type SphyraenaArgs struct {
	SessionIDGenerator *session.SessionIDGenerator
	SecretGenerator    *secret.Generator
	SessionServer      session.SessionServer
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
func New(args *SphyraenaArgs) *Sphyraena {
	supervisor := suture.NewSimple("sphyraena root supervisor")

	if args == nil {
		args = &SphyraenaArgs{}
	}

	if args.SessionIDGenerator == nil {
		args.SessionIDGenerator = session.NewSessionIDGenerator(128, nil)
	}
	supervisor.Add(args.SessionIDGenerator)
	if args.SecretGenerator == nil {
		args.SecretGenerator = secret.NewGenerator(128)
	}
	supervisor.Add(args.SecretGenerator)
	if args.SessionServer == nil {
		args.SessionServer = session.NewRAMServer(
			args.SessionIDGenerator, args.SecretGenerator,
			&session.RAMSessionSettings{time.Minute * 180, nil})
	}

	ctx := context.NewSphyraenaState(args.SessionServer, nil)
	r := router.New(ctx)

	return &Sphyraena{
		supervisor,
		ctx,
		r,
	}
}
