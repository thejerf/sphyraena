package utf8stream

import (
	"encoding/json"
	"fmt"

	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/strest"
)

func (s *UTF8Stream) Serve() error {
	go func() {
		for {
			outgoing, ok := <-s.toUser
			if !ok {
				fmt.Println("Terminating send loop")
				return
			}

			outbytes, err := json.Marshal(outgoing)
			if err != nil {
				// FIXME: Logging must go somewhere
				continue
			}
			s.sd.Send(string(outbytes))
		}
	}()

	for {
		msg, err := s.sd.Receive()
		fmt.Println(msg, err)
		if err != nil {
			// FIXME: error should go somewhere if it's not EOF
			close(s.fromUser)
			return err
		}

		// get the first byte to see how far to read
		// FIXME: Shield against illegal array reads
		len := msg[0]
		ty := string(msg[1 : 1+len])
		msg = msg[1+len:]

		switch ty {
		case "new_stream":
			fmt.Println("Seeing a new request come in:", string(msg))
			// FIXME: Rename this to stream request or something, it's not HTTP
			httpreq := HTTPRequest{}
			err := json.Unmarshal(msg, &httpreq)
			if err != nil {
				// FIXME: Do something better
				fmt.Println("Error unmarshaling msg:", err)
				continue
			}

			r, err := httpreq.ToRequest()
			if err != nil {
				// FIXME: do something better
				fmt.Println("Error converting to request:", err)
				continue
			}

			req := request.FromStream(
				s.session,
				s.stream,
				func(srr request.StreamRequestResult) {
					err := sendJSON(s, StreamMessage{
						Type:        "new_stream_response",
						ID:          httpreq.RequestID,
						Data:        srr,
						SubstreamID: srr.SubstreamID,
					})
					if err != nil {
						// FIXME: Log better
						fmt.Println("Couldn't send stream response:", err)
					}
				},
			)
			req.SphyraenaState = s.ss
			req.Request = r

			fmt.Println("Using router:", s.router)
			go s.router.RunStreamingRoute(req)

		default:
			fmt.Println("Unknown request type:", ty)
		}
	}
}

// FIXME: Is this already defined somewhere?
// FIXME: Namespace the request IDs so we can tell the difference between them.

type StreamMessage struct {
	Type        string             `json:"type"`
	ID          uint64             `json:"response_to,omitempty"`
	SubstreamID strest.SubstreamID `json:"substream_id"`
	Data        interface{}        `json:"data"`
}

func sendJSON(s *UTF8Stream, data interface{}) error {
	marshaled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Println("Sending response:", string(marshaled))
	return s.sd.Send(string(marshaled))
}
