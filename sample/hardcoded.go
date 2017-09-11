package sample

import (
	"fmt"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

// TODO: As the simplest possible StREST request, this file should be
// carefully observed for anything we can possibly be rid of.

// HardCoded implements the simplest possible StREST responder, which does
// not stream, or do anything else, except return a simple page with the
// content as hardcoded by its definition.
func HardCoded(content, mimeType string) context.Handler {
	return &hardcoded{content, mimeType}
}

type hardcoded struct {
	content string
	mime    string
}

func (hc *hardcoded) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, context *context.Context) {
	fmt.Println("In servestreaming")
	rw.Header().Set("Content-Type", hc.mime)
	rw.Write([]byte(hc.content))
}

func (hc *hardcoded) MayStream() bool {
	return false
}
