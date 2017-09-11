package sockjs

import (
	"fmt"

	"github.com/thejerf/sphyraena/context"
	"gopkg.in/igm/sockjs-go.v2/sockjs"
)

func StreamingRESTHandler() context.Handler {
	fmt.Println("In streamingRESTHandler")
	return context.NetHTTPHandler{sockjs.NewHandler("/socket", sockjs.DefaultOptions, echoHandler)}
}

func echoHandler(session sockjs.Session) {
	fmt.Println("Entering echo handler")
	for {
		if msg, err := session.Recv(); err == nil {
			fmt.Println("Got SockJS message:", msg)
			session.Send(msg)
			continue
		}
		break
	}
}
