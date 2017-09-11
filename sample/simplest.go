package sample

import (
	"strconv"
	"time"

	"github.com/thejerf/sphyraena/context"
	"github.com/thejerf/sphyraena/sphyrw"
)

// CounterOut is the "hello world" of outgoing-only Streaming REST,
// emitting a stream of incrementing integers.
func CounterOut(rw *sphyrw.SphyraenaResponseWriter, context *context.Context) {
	val := uint64(0)
	_, _ = rw.Write([]byte(strconv.FormatUint(val, 10)))

	stream, err := context.SubstreamToUser()

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

// Number is the "hello world" of bidirectional Streaming REST, emitting a
// series of incrementing numbers over time, and accepting incoming numbers
// to manipulate that number back from the user.
// func Number(rw *sphyrw.SphyraenaResponseWriter, context *context.Context) {
// 	val := uint64(0)

// 	initial := context.FormValue("initial")
// 	if initial != "" {
// 		parsed, err := strconv.ParseUint(initial, 10, 64)
// 		if err == nil {
// 			val = parsed
// 		}
// 	}

// 	_, _ = rw.Write([]byte(strconv.FormatUint(val, 10)))

// 	stream, err := context.Substream()
// 	ticker := time.NewTicker(time.Second)
// 	defer ticker.Stop()

// 	for {

// 	}
// }
