package request

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/thejerf/sphyraena/strest"
)

// FIXME: Eventually needs a context

type StreamRequest struct {
	*SphyraenaState
	*RouteResult
	session session.Session

	arguments []byte
}

// A StreamHandler implements something that returns a stream handler, and
// will be run in a separate goroutine to handle the stream.
type StreamHandler interface {
	HandleStream(*StreamRequest)
}

// Request methods for dealing with streams.

func (c *Request) getStream() (*strest.Stream, error) {
	if c.currentStream == nil {
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

	return stream.Substream()
}
