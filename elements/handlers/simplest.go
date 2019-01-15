package handlers

import (
	"fmt"
	"time"

	"github.com/thejerf/sphyraena/request"
)

// CounterOut is the "hello world" of outgoing-only Streaming REST,
// emitting a stream of incrementing integers.
func CounterOut(req *request.Request) {
	fmt.Println("Starting counter out")
	val := uint64(0)

	stream, err := req.SubstreamToUser()

	req.StreamResponse(request.StreamRequestResult{
		SubstreamID: stream.SubstreamID(),
	})

	if err == nil {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			val++
			<-ticker.C
			err := stream.Send(val)
			fmt.Println("sent", val)
			if err != nil {
				return
			}
		}
	} else {
		fmt.Println("Couldn't stream:", err)
	}
}
