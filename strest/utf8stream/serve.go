package utf8stream

import (
	"encoding/json"
	"fmt"
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
		case "request":
			fmt.Println("Seeing a new request come in:", string(msg))
		default:
			fmt.Println("Unknown request type:", ty)
		}
	}
}
