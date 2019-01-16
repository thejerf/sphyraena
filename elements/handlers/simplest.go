package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/strest"
)

// CounterOut is the "hello world" of outgoing-only Streaming REST,
// emitting a stream of incrementing integers.
func CounterOut(req *request.Request) {
	fmt.Println("Starting counter out")
	val := uint64(0)

	stream, err := req.SubstreamToUser()
	if err != nil {
		req.StreamResponse(request.StreamRequestResult{
			Error:     err.Error(),
			ErrorCode: 500,
		})
		return
	}

	req.StreamResponse(request.StreamRequestResult{
		SubstreamID: stream.SubstreamID(),
	})

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
}

// InteractiveCounterOut is the "hello world" of a bi-directional stream,
// emitting a stream of incrementing integers, and accepting incoming
// integers as things to add or subtract from the stream
func InteractiveCounterOut(req *request.Request) {
	fmt.Println("Starting interactive counter out")
	val := int64(0)

	stream, err := req.Substream()
	spew.Dump(stream)

	req.StreamResponse(request.StreamRequestResult{
		SubstreamID: stream.SubstreamID(),
	})

	if err != nil {
		req.StreamResponse(request.StreamRequestResult{
			Error:     err.Error(),
			ErrorCode: 500,
		})
		fmt.Println("Couldn't initiate stream:", err)
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	fromUser, toUser := stream.RawChans()
	maybeToUser := toUser
	haveMsg := false
	msg := strest.EventToUser{
		Source: stream.SubstreamID(),
		Close:  false,
		Type:   "event",
	}

	for {
		if haveMsg {
			maybeToUser = toUser
			msg.Message = val
		} else {
			maybeToUser = nil
		}

		select {
		case <-ticker.C:
			val++
			haveMsg = true

		case maybeToUser <- msg:
			haveMsg = false

		case incoming, ok := <-fromUser:
			if !ok {
				fmt.Println("Terminating ioc due to stream close")
				return
			}

			fmt.Println("Incoming message from user found")
			var msgcontent int64
			err := json.Unmarshal(incoming.JSON, &msgcontent)
			if err != nil {
				// FIXME: Need better logging. Perhaps a default error-like return?
				fmt.Println("Error unmarshaling message:", err)
				continue
			}
			val += msgcontent
		}
	}
}
