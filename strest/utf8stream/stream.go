package utf8stream

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/thejerf/sphyraena/identity/session"
	"github.com/thejerf/sphyraena/request"
	"github.com/thejerf/sphyraena/router"
	"github.com/thejerf/sphyraena/strest"
)

// A UTF8Stream is a streaming client that uses a UTF8StreamDriver to drive
// a Sphyraena stream.
type UTF8Stream struct {
	sd       UTF8StreamDriver
	toUser   chan strest.EventToUser
	fromUser chan strest.EventFromUser
	session  session.Session
	stream   *strest.Stream
	ss       *request.SphyraenaState
	router   *router.SphyraenaRouter
}

// FIXME: Document EXACTLY what this is.

// NewUTF8Stream returns a constructed UTF8Stream object. This also begins
// executing the corresponding goroutines.
//
// sd is a StreamDriver that is hooked up and ready to start streaming.
// The SphyraenaRouter is the top-level router for the requests. identity
// is the known identity of the current stream.
func NewUTF8Stream(
	sd UTF8StreamDriver,
	sess session.Session,
	stream *strest.Stream,
	ss *request.SphyraenaState,
	router *router.SphyraenaRouter,
) *UTF8Stream {
	return &UTF8Stream{
		sd,
		make(chan strest.EventToUser),
		make(chan strest.EventFromUser),
		sess,
		stream,
		ss,
		router,
	}
}

// Channels implements the strest.ExternalStream interface, allowing this
// to be hooked up to a Stream.
func (s *UTF8Stream) Channels() (chan strest.EventToUser, chan strest.EventFromUser) {
	return s.toUser, s.fromUser
}

// A UTF8StreamDriver corresponds to a stream that can send and receive
// discrete blocks of UTF8 text. These streams should generally not be used
// for multi-megabyte messages, though it depends on the details of the
// stream type.
//
// Sphyraena ships with a sockjs-based Websocket-type streamer, and a
// sample length-delimited string on an arbitrary reader & writer sample.
type UTF8StreamDriver interface {
	// Receive one text frame.
	Receive() ([]byte, error)

	// Send one text frame.
	Send(string) error

	// Close the stream
	Close() error
}

// HTTPRequest represents the raw input coming in from the client. We
// have to chew on the input to turn it into something that matches
// what http.Request is looking for.
//
// This is public for the JSON encoder. FIXME: Move to internal? IIRC
// internal stuff can be "public" and seen by the JSON encoder but not seen
// in the godoc.
type HTTPRequest struct {
	Method string `json:"method"`

	URL string `json:"url"`

	// assume HTTP/1.1
	Header http.Header `json:"header"`
	Body   string      `json:"body"`

	RequestID uint64 `json:"request_id"`
}

// ToRequest turns an incoming stream request into an HTTP request that can
// be routed and handled.
func (hr *HTTPRequest) ToRequest() (*http.Request, error) {
	req := new(http.Request)
	req.Method = hr.Method
	if hr.URL != "" {
		url, err := url.Parse(string(hr.URL))
		if err != nil {
			return nil, err
		}
		req.URL = url
	}
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	req.Header = hr.Header
	req.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(hr.Body)))

	return req, nil
}
