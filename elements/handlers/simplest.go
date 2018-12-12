package handlers

import (
	"strconv"
	"time"

	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/sphyrw"
)

// CounterOut is the "hello world" of outgoing-only Streaming REST,
// emitting a stream of incrementing integers.
func CounterOut(rw *sphyrw.SphyraenaResponseWriter, req *request.Request) {
	val := uint64(0)
	_, _ = rw.Write([]byte(strconv.FormatUint(val, 10)))

	stream, err := req.SubstreamToUser()

	rw.Finish()

	if err == nil {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			val++
			<-ticker.C
			err := stream.Send(val)
			if err != nil {
				return
			}
		}
	}
}
