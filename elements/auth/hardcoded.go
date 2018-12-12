package auth

import (
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/sphyrw"
)

// HardCodedResponse implements the simplest possible StREST responder, which does
// not stream, or do anything else, except return a set of bytes with the
// content as hardcoded by its definition. It claims no streaming
// capability, of course.
func HardCodedResponse(content, mimeType string) request.Handler {
	return &hardcoded{content, mimeType}
}

type hardcoded struct {
	content string
	mime    string
}

func (hc *hardcoded) ServeStreaming(rw *sphyrw.SphyraenaResponseWriter, req *request.Request) {
	rw.Header().Set("Content-Type", hc.mime)
	rw.Write([]byte(hc.content))
}

func (hc *hardcoded) MayStream() bool {
	return false
}
