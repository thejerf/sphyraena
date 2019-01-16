package strest

import "encoding/json"

// A lot of the core streaming logic between the three substreams are the
// same. close in particular is shared between all three. Also, it really
// helps the Stream itself to have a concrete type that it can use to do
// the things it wants.
type substream struct {
	// The stream owns the sending end of fromUser, so only it can close
	// it.
	toUser   chan EventToUser
	fromUser chan json.RawMessage

	substreamID SubstreamID
	// If this is true, the substream can receive messages from the
	// user. If false, the Stream will never send to this substream,
	// instead immediately erroring out the send call with a CloseSubstream
	// response.
	//
	// The symmetric value is not necessary, as it is enforced by the type
	// system. (We can't control whether the remote client attempts to send
	// via the local type system.)
	canReceive bool

	// This is owned by the substream. It would be a project to further
	// work on the substream to make it thread-safe on its own.
	closed bool
}

func (ss *substream) SubstreamID() SubstreamID {
	return ss.substreamID
}

func (ss *substream) close() error {
	if ss.closed {
		return ErrClosed
	}
	ss.closed = true

	msg := ss.closeMessage()
	for {
		select {
		case ss.toUser <- msg:
			return nil
		// we're closing, which means our handler is done expecting
		// messages. Should the stream send any more, just drain it.
		case _, ok := <-ss.fromUser:
			if ok {
				// drain, continue around for loop
			} else {
				// upstream has closed, we're closed too
				return ErrClosed
			}
		}
	}
}

func (ss *substream) message(msg interface{}) EventToUser {
	return EventToUser{ss.substreamID, false, msg, "event"}
}

func (ss *substream) closeMessage() EventToUser {
	return EventToUser{ss.substreamID, true, nil, "event"}
}

// A SendOnlySubstream is a Substream that only has Sending
// capabilities. In particular, any attempt by a client to send to this
// substream will be immediately met with a CloseSubstreamNoSend response,
// with no communication to the substream in question.
type SendOnlySubstream struct {
	*substream
}

// Send sends a message to the user.
//
// The possible error is ErrClosed. If this is received, this substream
// will never send again.
func (sos *SendOnlySubstream) Send(msg interface{}) error {
	// as this is only safe on a SendOnlySubstream, we implement it here,
	// instead of in the substream type.
	if sos.closed {
		return ErrClosed
	}
	select {
	case sos.toUser <- EventToUser{sos.substreamID, false, msg, "event"}:
		return nil
	case _, _ = <-sos.fromUser:
		// the only way this can happen for a SendOnlySubstream is if the
		// stream is shutting down while we're trying to send, in which
		// case, we're closed.
		sos.closed = true
		return ErrClosed
	}
}

// Close closes the substream. All further send attempts will fail with
// ErrClosed.
func (sos *SendOnlySubstream) Close() error {
	return sos.close()
}

// Message returns the properly-formatted EventToUser that can be used with
// the channels returned by RawChans to send the given message.
func (sos *SendOnlySubstream) Message(msg interface{}) EventToUser {
	return sos.message(msg)
}

// CloseMessage returns the properly-formatted EventToUser that, when
// sent on the channel given by RawChans, will close this substream.
func (sos *SendOnlySubstream) CloseMessage() EventToUser {
	return sos.closeMessage()
}

// RawChans is documented under Substream.RawChans.
//
// Please note warning about correct use. It is always wrong to only use
// one of these channels in a select.
func (sos *SendOnlySubstream) RawChans() (<-chan json.RawMessage, chan<- EventToUser) {
	return sos.fromUser, sos.toUser
}

// A ReceiveOnlySubstream is a Substream that can only receieve.
type ReceiveOnlySubstream struct {
	*substream
}

// Close is documented under Substream.Close.
func (ros *ReceiveOnlySubstream) Close() error {
	return ros.close()
}

// ReceiveChan returns the channel that the Stream will send events in to,
// if you prefer to use this substream as part of a select statement
// instead of using the .Receive method.
func (ros *ReceiveOnlySubstream) ReceiveChan() <-chan json.RawMessage {
	return ros.fromUser
}

// Receive will receive one message from the remote user.
//
// If an error is returned, no further Receive calls will work.
func (ros *ReceiveOnlySubstream) Receive() (interface{}, error) {
	// As this is only safe in a ReceiveOnlySubstream, we implement this
	// here instead of in the substream.
	if ros.closed {
		return nil, ErrClosed
	}

	msg, ok := <-ros.fromUser
	if ok {
		return msg, nil
	}
	ros.closed = true
	return nil, ErrClosed
}

// An Substream is a bi-directional communicator with a remote stream.
//
// The Substream's communication with its parent stream is threadsafe,
// however, the substream itself is not; in particular, attempting to send
// messages in one goroutine while closing it in another is a race
// condition. If your code is run with -race, this produces complaints
// about not synchronizing on the private "closed" boolean in a substream.
//
// To use this stream, you must fetch its channels and use them properly in
// your select calls. The two legal patterns are:
//
//  1. Use the chan interface{} only, to receive possible incoming
//     messages.
//  2. Use both the chan interface{} and chan EventToUser, to send and
//     possibly receive a message.
//
// It is a GUARANTEED BUG to send on the EventToUser channel without
// also listening to the interface{} channel, as the remote stream may
// close and never pick up your message, producing a goroutine leak.
//
// This also means that when using a bi-directional substream, you may
// receive any number of incoming messages whenever you attempt to send.
type Substream struct {
	*substream
}

// I'll drop this in the comments here, we'll see if we have to raise it up
// to the doc level later: It's really tempting to try to work out an
// interface that all of these conform to so one can somehow speak of
// "streams" in general, but it's a false hope. A legal "Send" function on
// a bidirectional stream would have to safely attempt to send the message,
// but it *could* fail because it instead *received* a message, in which
// case it would have to deal with that, and by the time you're done
// spec'ing it all out all you've done is inner-platform an inferior
// "select" statement. A receive-only stream still has a channel going back
// to the Stream itself, which it can close to indicate that the substream
// is closed, but offering that to the user means that it could *send* on
// that channel too, and then the Stream would have to look up whether it
// should do anything with it, basically creating an if block and more
// complicated code for no gain. What may at first glance look like a
// haphazard collection of methods and semantics is actually
// carefully-crafted to enable *easy and correct* usage. Pretty much the
// only thing you can do differently is simply get rid of the send-only and
// receive-only substreams, but I think they're a valuable shortcut because
// they are such common cases.

// Close will close the substream properly, indicating to the Stream that
// this substream is closed. This will close both directions of the
// Substream.
//
// Once a stream is closed, it is invalid to send anything else.
func (ss *Substream) Close() error {
	return ss.close()
}

// Message returns the proper EventToUser to send a message to
// the end user using the raw channels directly.
func (ss *Substream) Message(msg interface{}) EventToUser {
	return ss.message(msg)
}

// CloseMessage returns the proper StreamEvent to indicate that a substream
// is closing, using the Substream.ToUser.
//
// Sending this message to the user will cause this substream to be closed
// by the parent stream. This allows you to put the closing of the stream
// in a select itself. Examples will presumably be forthcoming once I have
// one that is not fake.
func (ss *Substream) CloseMessage() EventToUser {
	return ss.closeMessage()
}

// RawChans returns the raw channels used to send and receive events from
// the user.
//
// It is preferable to use the .Send function if you are simply sending
// something, but you can use this to have the stream channels partipate in
// some other select statement. However, you MUST always use BOTH channels,
// becuase the way that you will be signaled that the stream is closed is
// that the FromUser channel will be closed. This is true even for
// send-only substreams. Failure to check the EventFromUser channel for
// being closed will cause hanging goroutine leaks.
func (ss *Substream) RawChans() (<-chan json.RawMessage, chan<- EventToUser) {
	return ss.fromUser, ss.toUser
}
