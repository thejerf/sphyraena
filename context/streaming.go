package context

import "github.com/thejerf/sphyraena/strest"

// Context methods for dealing with streams.

func (c *Context) getStream() (*strest.Stream, error) {
	if !c.RunningAsGoroutine {
		return nil, strest.ErrNoStreamingContext
	}

	if c.currentStream == nil {
		stream, err := c.session.NewStream()
		if err != nil {
			return nil, err
		}

		c.currentStream = stream
	}

	return c.currentStream, nil
}

func (c *Context) StreamID() (strest.StreamID, error) {
	s, err := c.getStream()
	if err != nil {
		return strest.StreamID(""), err
	}
	return s.ID(), nil
}

func (c *Context) SubstreamFromUser() (*strest.ReceiveOnlySubstream, error) {
	stream, err := c.getStream()
	if err != nil {
		return nil, err
	}

	return stream.SubstreamFromUser()
}

func (c *Context) SubstreamToUser() (*strest.SendOnlySubstream, error) {
	stream, err := c.getStream()
	if err != nil {
		return nil, err
	}

	return stream.SubstreamToUser()
}

func (c *Context) Substream() (*strest.Substream, error) {
	stream, err := c.getStream()
	if err != nil {
		return nil, err
	}

	return stream.Substream()
}
