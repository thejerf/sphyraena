package sockjs

import (
	"context"
	"fmt"

	sockjssrv "github.com/igm/sockjs-go/sockjs"
	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/router"
	"github.com/thejerf/sphyraena/sphyrw"
	"github.com/thejerf/sphyraena/strest/utf8stream"
)

var DefaultOptions = sockjssrv.DefaultOptions

// As it happens, a sockjs.Session is a utf8stream.UTF8StreamDriver, so we
// don't require a wrapper struct.

type sockjskey string

// FIXME: Update the name here

func StreamingRESTHandler(
	prefix string,
	sr *router.SphyraenaRouter,
	ss session.SessionServer,
	options sockjssrv.Options,
) request.HandlerFunc {
	sockjsHandler := sockjssrv.NewHandler(prefix, options,
		func(sjs sockjssrv.Session) {
			fmt.Println("Serving session handler")
			origReq := sjs.Request()
			reqURL := origReq.URL
			values := reqURL.Query()
			streamID := values.Get("stream_id")

			origSphyReq := origReq.Context().Value(sockjskey("orig_sphy_req")).(*request.Request)
			mySession := origSphyReq.Session()
			fmt.Printf("My session: %#v\n", mySession)
			stream, err := mySession.GetStream([]byte(streamID))
			if err != nil {
				// FIXME: Log properly
				fmt.Println("Failed to get stream", streamID, ":", err)
				return
			}

			u8s := utf8stream.NewUTF8Stream(
				sockJSDriver{sjs},
				origSphyReq.Session(),
				stream,
				origSphyReq.SphyraenaState,
				sr,
			)

			if err != nil {
				// FIXME: Log properly
				fmt.Println("Failed to find stream |", string(streamID), "|")
				return
			}

			// now take the stream over
			stream.SetExternalStream(u8s)

			fmt.Println("Falling off the bottom of the sockjs handler")

			u8s.Serve()
		})

	handler := func(
		rw *sphyrw.SphyraenaResponseWriter,
		req *request.Request,
	) {
		fmt.Println("Got request for socket")
		desiredContext := context.WithValue(
			req.Context(),
			sockjskey("orig_sphy_req"),
			req,
		)

		req.Request = req.Request.WithContext(desiredContext)
		sockjsHandler.ServeHTTP(rw, req.Request)
	}

	return request.HandlerFunc(handler)
}

type sockJSDriver struct {
	sess sockjssrv.Session
}

func (sjd sockJSDriver) Close() error {
	return sjd.sess.Close(0, "closed")
}

func (sjd sockJSDriver) Receive() ([]byte, error) {
	s, err := sjd.sess.Recv()
	return []byte(s), err
}

func (sjd sockJSDriver) Send(s string) error {
	return sjd.sess.Send(s)
}

func echoHandler(session sockjssrv.Session) {
	fmt.Println("Entering echo handler")
	for {
		if msg, err := session.Recv(); err == nil {
			fmt.Println("Got SockJS message:", msg)
			fmt.Printf("Session type: %T\n", session)
			session.Send(msg)
			continue
		}
		break
	}
}
