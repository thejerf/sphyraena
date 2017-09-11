package router

import (
	"fmt"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

// runInGoroutine contains all the code to run an HTTP handler in its own
// goroutine. This allows that goroutine to indicate to the goroutine
// running in the http server that this specific request is terminated, but
// the goroutine can continue running to emit events.
func (sr *SphyraenaRouter) runInGoroutine(handler context.Handler, rw *sphyrw.SphyraenaResponseWriter, ctx *context.Context) {
	// let's do a basic first-cut pass of simply running this in a
	// goroutine before we get fancy

	done := make(chan struct{})
	rw.SetDoneChan(done)
	ctx.RunningAsGoroutine = true

	fmt.Println("Beginning goroutine serve")
	go func() {
		fmt.Println("Goroutine running")
		handler.ServeStreaming(rw, ctx)
		fmt.Println("Goroutine serve complete, signaling")
		done <- struct{}{}
	}()

	<-done
	fmt.Println("Goroutine returned control to the http thread, done serving")
}
