package router

import (
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/sphyrw"
)

type PanicInfo struct {
	panicreason interface{}
}

// runInGoroutine contains all the code to run an HTTP handler in its own
// goroutine. This allows that goroutine to indicate to the goroutine
// running in the http server that this specific request is terminated, but
// the goroutine can continue running to emit events.
func (sr *SphyraenaRouter) runInGoroutine(handler request.Handler, rw *sphyrw.SphyraenaResponseWriter, req *request.Request) {
	// let's do a basic first-cut pass of simply running this in a
	// goroutine before we get fancy

	done := make(chan *PanicInfo)
	req.RunningAsGoroutine = true

	go func() {
		defer func() {
			// FIXME: We'll have to manually snapshot the stack trace here
			if r := recover(); r != nil {
				done <- &PanicInfo{r}
			}
		}()
		handler.ServeStreaming(rw, req)
		done <- nil
	}()

	panicReason := <-done
	if panicReason != nil {
		panic(panicReason.panicreason)
	}
}
