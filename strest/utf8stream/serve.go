package utf8stream

import (
	"encoding/json"
	"fmt"

	"github.com/thejerf/sphyraena/request"
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
		}

		// get the first byte to see how far to read
		// FIXME: Shield against illegal array reads
		len := msg[0]
		ty := string(msg[1 : 1+len])
		msg = msg[1+len:]

		switch ty {
		case "new_stream":
			fmt.Println("Seeing a new request come in:", string(msg))
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

				},
			)
			req.SphyraenaState = &s.ss
			req.Request = r

			go s.router.RunStreamingRoute(req)

		default:
			fmt.Println("Unknown request type:", ty)
		}
	}
}
