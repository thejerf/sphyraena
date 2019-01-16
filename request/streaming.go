package request

import (
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/thejerf/sphyraena/strest"
)

// FIXME: Eventually needs a context

type StreamRequestResult struct {
	SubstreamID strest.SubstreamID `json:"substream_id,omitempty"`
	Error       string             `json:"error,omitempty"`
	ErrorCode   int                `json:"error_code,omitempty"`
}

// A StreamHandler implements something that returns a stream handler, and
// will be run in a separate goroutine to handle the stream.
type StreamHandler interface {
	HandleStream(*Request)
}

type StreamHandlerFunc func(*Request)

func (shf StreamHandlerFunc) HandleStream(req *Request) {
	shf(req)
}

// Request methods for dealing with streams.

func (c *Request) getStream() (*strest.Stream, error) {
	if c.currentStream == nil {
		fmt.Printf("Getting stream from session of type %T\n", c.session)
		stream, err := c.session.NewStream()
		if err != nil {
			return nil, err
		}

		c.currentStream = stream
	}

	return c.currentStream, nil
}

// Gets an authenticated stream ID suitable for use by the client side to
// request a stream.
func (c *Request) StreamID() (strest.StreamID, error) {
	s, err := c.getStream()
	if err != nil {
		return strest.StreamID(""), err
	}
	streamID := []byte(string(s.ID()))
	authedStreamID, err := c.session.Authenticate(streamID)
	if err != nil {
		return strest.StreamID(""), err
	}
	return strest.StreamID(string(authedStreamID)), nil
}

// FIXME: This should issue the StreamResponse automatically

func (c *Request) SubstreamFromUser() (*strest.ReceiveOnlySubstream, error) {
	stream, err := c.getStream()
	if err != nil {
		return nil, err
	}

	return stream.SubstreamFromUser()
}

func (c *Request) SubstreamToUser() (*strest.SendOnlySubstream, error) {
	stream, err := c.getStream()
	if err != nil {
		return nil, err
	}

	return stream.SubstreamToUser()
}

func (c *Request) Substream() (*strest.Substream, error) {
	stream, err := c.getStream()
	if err != nil {
		return nil, err
	}

	fmt.Println("****\n****\n*****\n\n*******")
	ss, err := stream.Substream()
	spew.Dump(ss)
	return ss, err
}
