/*

Package strest contains the "streaming rest" support code.

FIXME: I have to separate out REST handlers and streaming handlers;
handling the same thing on the same URL will be handled by the router.
So this package should be the streaming support package. But it needs to be
renamed from "strest" to just "streaming"; there is no longer a distinction.

As of this writing, it is not clear exactly what this package really is.
The code in it is definitely good and needs to live somewhere, but the
concept of a "stream REST resource" is continuing to be boiled farther
and farther down, meaning that there may not be anything left by the end,
and this may turn into some sort of "stream" package.

*/
package strest

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

// FIXME: A stream needs to be able to determine when its session is over
// and terminate if the session is still expired.

// ErrClosed is returned when either the stream or the substream being used
// is closed.
var ErrClosed = errors.New("stream closed")

// ErrNoStreamingContext is returned when a stream is requested, but there
// is no streaming context available.
//
// FIXME: Better name?
var ErrNoStreamingContext = errors.New("no streaming context available")

type substreamret struct {
	ss  *substream
	err error
}

// This file defines a "stream" abstraction which, when passed in as part
// of a Streaming REST request, allows you to interact with streamed events
// and such easily.

// A SubstreamID represents the ID of a stream within the conglomeration of
// Streaming REST resources represented by a Stream.
//
// Note a given SubstreamID is only defined within a Stream.
type SubstreamID uint32

// A StreamID identifies the given stream, used by sessions to match
// streams to Stream objects when necessary.
type StreamID string

// EventFromUser represents an incoming event from whatever is concretely
// instantiating the stream.
type EventFromUser struct {
	Dest    SubstreamID `json:"dest"`
	Close   bool        `json:"close,omitempty"`
	Message interface{} `json:"message,omitempty"`
}

// EventToUser represents an outgoing event from whatever is concretely
// instantiating the stream.
type EventToUser struct {
	Source  SubstreamID `json:"source"`
	Close   bool        `json:"close,omitempty"`
	Message interface{} `json:"message,omitempty"`
}

// An ExternalStream is something from which the requisite channels can
// be extracted.
//
// The channels MUST NOT be nil. If you need to indicate some sort of error
// condition even before the stream is using this ExternalStream, the
// FromUser may come "pre-closed", which looks no different to the Stream
// than if it was closed later.
//
// Structs implementing this interface MUST return the same channels for
// every invocation.
type ExternalStream interface {
	Channels() (chan EventToUser, chan EventFromUser)
}

// ChannelsStream is a struct that implements the ExternalStream interface
// if given a chan EventToUser and chan EventFromUser.
type ChannelsStream struct {
	ToUser   chan EventToUser
	FromUser chan EventFromUser
}

// Channels implements the ExternalStream interface.
func (cs ChannelsStream) Channels() (chan EventToUser, chan EventFromUser) {
	return cs.ToUser, cs.FromUser
}

// FIXME: Eventually, we need a sort of "stream stub" that can receive all
// the methods and atomically start up a stream only when requested, since
// we don't want to start up a stream that will not be used. For now, we
// just automatically start them up.

// A Stream represents a single, coherent stream to and from a given user,
// which is responsible for managing the aggregation of many substreams
// into this stream.
//
// The Stream is not responsible for communication; that is done by
// implementing something that can consume and produce EventToUser and
// EventFromUser structs, and calling the SetExternalStream method.
//
// Future warning: At the moment, it is permitted to call SetExternalStream
// more than once. This will likely be removed in the future as there is
// probably no practical safe way to transfer streams without potentially
// losing messages, but we'll see how that goes.
type Stream struct {
	streamMembers map[SubstreamID]*substream
	commands      chan streamCommand

	fromSubstreamToUser chan EventToUser
	fromUser            chan EventFromUser
	toUser              chan EventToUser

	id              StreamID
	nextSubstreamID SubstreamID

	closedMutex sync.Mutex
	closed      bool

	logger func(string, ...interface{})
}

// NewStream returns a new stream.
//
// This will start a goroutine handling the stream. .Close() must be called
// to terminate this goroutine.
func NewStream(id StreamID) *Stream {
	s := &Stream{
		id:                  id,
		streamMembers:       map[SubstreamID]*substream{},
		fromSubstreamToUser: make(chan EventToUser),
		commands:            make(chan streamCommand),
		nextSubstreamID:     SubstreamID(1), // FIXME: Randomize or something?
		fromUser:            nil,
		toUser:              nil,
		logger:              log.Printf,
	}
	go s.serve()
	return s
}

// ID returns a copy of the StreamID of this stream.
func (s *Stream) ID() StreamID {
	return s.id
}

func (s *Stream) serve() {
	// FIXME: Some sort of timeout is probably called for.
	defer func() {
		if r := recover(); r != nil {
			s.logger("Stream somehow actually crashed: %v", r)
		}
		s.cleanup()
	}()

	fmt.Println("In stream serve")

	sendingToUser := chan EventToUser(nil)
	msgs := []*EventToUser{}
	nilEventToUser := &EventToUser{}
	var nextMessage *EventToUser
	for {
		fmt.Println("Select loop")
		// This adapts the fairly standard idiom in Go of setting a
		// possibly-interesting channel in a select to nil if it isn't
		// interesting right now to include the message to send on that
		// channel, if possible.
		if len(msgs) == 0 {
			nextMessage = nilEventToUser
			sendingToUser = nil
		} else {
			nextMessage = msgs[0]
			sendingToUser = s.toUser
		}

		select {
		case m := <-s.commands:
			switch msg := m.(type) {
			case getSubstream:
				ssID := s.nextSubstreamID
				s.nextSubstreamID++
				ss := &substream{
					substreamID: ssID,
					toUser:      s.fromSubstreamToUser,
					fromUser:    make(chan interface{}),
					canReceive:  msg.canReceive,
				}
				s.streamMembers[ssID] = ss
				msg.ss <- substreamret{ss, nil}
			case setExternalStream:
				s.fromUser = msg.fromUser
				s.toUser = msg.toUser
			case unsetExternalStream:
				if s.fromUser == msg.fromUser && s.toUser == msg.toUser {
					s.fromUser = nil
					s.toUser = nil
				}
			case stop:
				return
			case dopanic:
				panic(msg.panicval)
			}
		case sendingToUser <- *nextMessage:
			// this makes it so that if messages are going out much slower
			// than they are being received, the common case, this slice
			// does not tend to itself generate garbage, re-using the
			// same slot over and over
			if len(msgs) == 1 {
				msgs = msgs[:0]
			} else {
				msgs = msgs[1:]
			}
		case m := <-s.fromSubstreamToUser:
			// FIXME: We need some sort of very high limit that says
			// this is just too much right now.
			if m.Close {
				ssID := m.Source
				ss, haveSS := s.streamMembers[ssID]
				if !haveSS {
					continue
				}
				close(ss.fromUser)
				delete(s.streamMembers, ssID)
				msgs = append(msgs, &m)
			} else {
				msgs = append(msgs, &m)
			}
		case incoming, ok := <-s.fromUser:
			if !ok {
				return
			}

			fmt.Printf("Received incoming message: %#v\n", incoming)

			dest := incoming.Dest
			ss, hasStream := s.streamMembers[dest]
			if !hasStream {
				msgs = append(msgs, &EventToUser{dest, true, nil})
				continue
			}

			if !ss.canReceive {
				// Eat the message, because the stream can't receive
				// it. Should some sort of message be sent back? This
				// technically fulfills the "contract" but is a bit
				// uninformative. On the other hand, it seems like this is
				// a user-end bug; why would you ever "send" to something
				// that isn't receiving? Since there's no "generic"
				// protocol defined here, what would that even mean?
				continue
			}

			if incoming.Close {
				close(ss.fromUser)
				delete(s.streamMembers, dest)
				continue
			}

			ss.fromUser <- incoming.Message
		}
	}
}

// Close terminates the Stream and its associated goroutine.
func (s *Stream) Close() error {
	return s.sendCommand(stop{})
}

func (s *Stream) cleanup() {
	// prevent any more messages from comming in on the command channel
	s.closedMutex.Lock()
	s.closed = true
	s.closedMutex.Unlock()

	// drain the command channel, to ensure nobody is waiting on
	// commands (since all command sends are protected by that mutex).
DRAIN_COMMANDS:
	for {
		select {
		case m := <-s.commands:
			switch msg := m.(type) {
			// handle the possible incoming messages that require
			// replies
			case getSubstream:
				msg.ss <- substreamret{nil, ErrClosed}
			default:
				// don't need to do anything for setExternalStream?
			}
		default:
			break DRAIN_COMMANDS
		}
	}

	for _, substream := range s.streamMembers {
		close(substream.fromUser)
	}
	// signal to whatever is handling the communication to the user that
	// the stream is closed for whatever reason.
	if s.toUser != nil {
		close(s.toUser)
	}
}

// commands are actually relatively rare; sync'ing on a mutex is not that
// big a deal since it won't happen often, and contention is quite unlikely.
func (s *Stream) sendCommand(sc streamCommand) error {
	s.closedMutex.Lock()
	nowClosed := s.closed
	s.closedMutex.Unlock()

	if nowClosed {
		return ErrClosed
	}

	s.commands <- sc
	return nil
}

// SetExternalStream accepts channels that are hooked up to some concrete
// communicatation mechanism, and will communicate with some user.
func (s *Stream) SetExternalStream(es ExternalStream) {
	toUser, fromUser := es.Channels()
	s.commands <- setExternalStream{toUser, fromUser}
}

// DisconnectExternalStream notifies the Stream that the given
// ExternalStream should no longer be sent messages.
//
// It is necessary to send the ExternalStream you are trying to disconnect
// so that the stream will not disconnect any other ExternalStreams, such
// as one that may have superceded this one.
func (s *Stream) DisconnectExternalStream(es ExternalStream) {
	toUser, fromUser := es.Channels()
	s.commands <- unsetExternalStream{toUser, fromUser}
}

// SubstreamToUser returns a Substream that can only be used to send to the
// user.
//
// (Limiting the stream in this way helps ensure that if the client code
// accidentally tries to send an event back up the substream, we don't end
// up blocking on trying to send to a channel.)
func (s *Stream) getSubstream(canReceive bool) (*substream, error) {
	if s == nil {
		return nil, ErrNoStreamingContext
	}
	c := make(chan substreamret)
	err := s.sendCommand(getSubstream{canReceive, c})
	if err != nil {
		return nil, err
	}
	ssret := <-c
	return ssret.ss, ssret.err
}

// SubstreamToUser returns a stream that can only be used to send to a
// given user. This can be more convenient than a full Substream because
// you can use the .Send method to send to a user without getting the raw
// channels out and using select, which can not be correctly implemented
// in this library for bi-directional streams.
func (s *Stream) SubstreamToUser() (*SendOnlySubstream, error) {
	ss, err := s.getSubstream(false)
	if err != nil {
		return nil, err
	}
	return &SendOnlySubstream{ss}, nil
}

// SubstreamFromUser returns a ReceiveOnlySubstream that can only be used
// to receive events from the user. Like SubstreamToUser, the resulting
// substream is more convenient because it can directly use the .Receive
// method without having to use the channels in a select statement, which
// can not be correctly implemented in this library for bi-directional
// streams.
func (s *Stream) SubstreamFromUser() (*ReceiveOnlySubstream, error) {
	ss, err := s.getSubstream(true)
	if err != nil {
		return nil, err
	}
	return &ReceiveOnlySubstream{ss}, nil
}

// Substream returns a Substream that can be used for bidirectional
// communication with the end-user, which must be used with Select.
func (s *Stream) Substream() (*Substream, error) {
	ss, err := s.getSubstream(true)
	if err != nil {
		return nil, err
	}
	return &Substream{ss}, nil
}

type Streams interface {
	SubstreamToUser() (*SendOnlySubstream, error)
	SubstreamFromUser() (*ReceiveOnlySubstream, error)
	Substream() (*Substream, error)
}
