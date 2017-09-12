package router

import (
	"fmt"
	"runtime/debug"

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
	panicChan := make(chan string)
	rw.SetDoneChan(done)
	ctx.RunningAsGoroutine = true

	// FIXME: Marshalling the panic over to the core routine isn't
	// necessary, and isn't even necessarily advisable; we should instead
	// copy the handling out of net/http

	// FIXME: Furthermore, we may just be better off hijacking the
	// connection ourselves, which could allow us to remove this entire
	// extra goroutine.
	go func() {
		defer func() {
			r := recover()
			if r != nil {
				stack := debug.Stack()
				p := fmt.Sprintf("Panic in streaming handler: %v\n\nStack:\n%s\n",
					r, string(stack))
				panicChan <- p
			}
		}()
		handler.ServeStreaming(rw, ctx)
		done <- struct{}{}
	}()

	select {
	case <-done:
		return
	case p := <-panicChan:
		panic(p)
	}
}
