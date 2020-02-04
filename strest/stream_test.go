package strest

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

// ***
// TEST SUPPORT CODE
// ***

func getTestStream() (*Stream, chan EventToUser, chan EventFromUser) {
	s := NewStream(StreamID(1))
	toUser := make(chan EventToUser)
	fromUser := make(chan EventFromUser)
	s.SetExternalStream(ChannelsStream{toUser, fromUser})
	return s, toUser, fromUser
}

func correctlySent(toUser chan EventToUser, ssID SubstreamID, msg interface{}) bool {
	streamEvent := <-toUser
	if streamEvent.Source != ssID {
		fmt.Println("in correctlySent, wrong source")
		return false
	}
	if streamEvent.Close {
		fmt.Println("in correctlySent, closed instead")
		return false
	}
	if streamEvent.Message != msg {
		fmt.Println("in correctlySent, wrong message found")
		return false
	}
	return true
}

func sampleEmitter(ss *SendOnlySubstream) {
	// Use Send once.
	_ = ss.Send(1)

	// Manually use the ToUserChan and Message functions
	_, toUser := ss.RawChans()
	// cheat during testing and just raw send, despite my docs; depend on
	// deadlock detection to pick up failures
	toUser <- ss.Message(2)

	_ = ss.Send(3)
}

// ***
// TEST CODE
// ***

func TestEmitting(t *testing.T) {
	s, _, _ := getTestStream()
	defer func() {
		_ = s.Close()
	}()

	if s.ID() != StreamID(1) {
		t.Fatal("Can't retrieve stream IDs")
	}

	sync := make(chan struct{})

	ss, _ := s.SubstreamToUser()

	go func() {
		r1 := correctlySent(s.toUser, ss.substreamID, 1)
		r2 := correctlySent(s.toUser, ss.substreamID, 2)
		r3 := correctlySent(s.toUser, ss.substreamID, 3)
		if r1 && r2 && r3 {
			sync <- struct{}{}
		}
		// else let deadlock tell us something is wrong
	}()

	sampleEmitter(ss)
	<-sync

	if !reflect.DeepEqual(ss.CloseMessage(),
		EventToUser{ss.substreamID, true, nil, "event"}) {
		t.Fatal("Close message not working for send-only substream")
	}

	ss.Close()
	err := ss.Send(0)
	if err != ErrClosed {
		t.Fatal("Can send on closed send-only stream")
	}

	err = ss.Close()
	if err != ErrClosed {
		t.Fatal("Can double-close a stream without getting an error")
	}
}

/*
func TestReceiving(t *testing.T) {
	s, _, fromUser := getTestStream()
	defer s.Close()

	ss, _ := s.SubstreamFromUser()
	go func() {
		for i := int(0); i < 3; i++ {
			// note this is actually simulating it coming from the user,
			// not directly poking the stream->substream channel
			fromUser <- EventFromUser{ss.substreamID, false, i}
		}
	}()

	_ = ss.ReceiveChan()

	for i := int(0); i < 3; i++ {
		msg, err := ss.Receive()
		if err != nil {
			t.Fatal("Got an error on what should be a clean receive")
		}
		if msg.(int) != i {
			t.Fatal("Somehow did not get the message from the user received")
		}
	}
	ss.Close()
	msg, err := ss.Receive()
	if msg != nil || err != ErrClosed {
		t.Fatal("Was able to receive a message post-closure")
	}

}

func TestUserClosesSubstream(t *testing.T) {
	s, _, fromUser := getTestStream()
	defer s.Close()

	ss, _ := s.SubstreamFromUser()
	ch := make(chan struct{})
	go func() {
		_, ok := <-ss.fromUser
		if !ok {
			ch <- struct{}{}
		}
	}()

	fromUser <- EventFromUser{ss.substreamID, true, nil}
	<-ch
	// if we make it here, the substreamFromUser was closed properly
}

func TestClosingNonexistantSubstreams(t *testing.T) {
	// This tests that getting a close after we already removed a substream
	// doesn't fail.
	s, _, _ := getTestStream()

	s.fromSubstreamToUser <- EventToUser{SubstreamID(50), true, nil}
	s.Close()
}

// This tests that a closed stream doesn't end up hanging every process
// that touches it by freezing on a channel that will never receive or something.
func TestClosingStream(t *testing.T) {
	s, toUser, fromUser := getTestStream()
	defer s.Close()

	ss, _ := s.SubstreamToUser()
	// this goroutine now owns the substream
	go sampleEmitter(ss)

	// Verify at least one message came from the emitter, to verify that
	// the sampleEmitter goroutine was started and is running
	if !correctlySent(s.toUser, ss.substreamID, 1) {
		t.Fatal("Did not get even one event")
	}

	// "the websocket was just closed or timed out or something"
	close(fromUser)
	_, _ = s.SubstreamToUser()

	// by sync'ing on the close of toUser, we can know that we're past the
	// point where the stream has closed, allowing us to test that the
	// sendCommand properly fails
	_, _ = <-toUser
	_, err := s.Substream()
	if err != ErrClosed {
		t.Fatal("Stream is unexpectedly not closed.")
	}
}

func TestFullSubstream(t *testing.T) {
	s, _, _ := getTestStream()
	defer s.Close()

	ss, _ := s.Substream()

	streamToUser, streamFromUser := ss.RawChans()
	if streamToUser == nil || streamFromUser == nil {
		t.Fatal("Raw chans doesn't work")
	}

	if !reflect.DeepEqual(ss.Message("moo"),
		EventToUser{ss.substreamID, false, "moo"}) {
		t.Fatal("message call not working on substream")
	}
	if !reflect.DeepEqual(ss.CloseMessage(),
		EventToUser{ss.substreamID, true, nil}) {
		t.Fatal("Close message not working for send-only substream")
	}
	ss.Close()
}

func TestUserSideSendingIllegally(t *testing.T) {
	s, toUser, fromUser := getTestStream()

	ss, err := s.SubstreamToUser()
	if err != nil {
		t.Fatal("Could not get substream to user")
	}

	// There's two illegal things the user can do:
	// 1. Send to a non-existant stream:
	fromUser <- EventFromUser{SubstreamID(4), false, "moo"}
	reply := <-toUser

	if !reply.Close {
		t.Fatal("Stream does not correctly send closeSubstream when speaking to nonexistant substream")
	}

	// 2. Send to a receive-only stream, in which case the message is eaten
	// without errors or deadlocks.
	fromUser <- EventFromUser{ss.substreamID, false, "moo"}
	s.Close()
}
*/

func TestCoverageDraining(t *testing.T) {
	// this is a bit hacky. I can not think of a way to reliably test the
	// draining functionality without a sleep in it. I think this is the
	// first time I've been defeated that way. If you can think of a way of
	// doing this reliably without a sleep, I'm all ears... but be sure it
	// works first!
	// Deliberately do not want the goroutine for the stream running
	s := &Stream{
		id:                  StreamID(0),
		streamMembers:       map[SubstreamID]*substream{},
		fromSubstreamToUser: make(chan EventToUser),
		commands:            make(chan streamCommand),
	}
	// note that according to the documentation, it is illegal to t.Fatal
	// in a goroutine other than the one calling this test
	// function. Therefore, anything that needs to be tested needs to get
	// back here.
	errors := make(chan error)
	complete := make(chan struct{})

	go func() {
		_, err := s.SubstreamToUser()
		errors <- err
		complete <- struct{}{}
	}()
	go func() {
		s.Close()
		complete <- struct{}{}
	}()

	// hopefully during this sleep, the previous two goroutines progress to
	// the point where they are trying to write to the channel. Since we
	// never started the stream's goroutine, that will never drain those commands.
	// Only the coverage graph can really tell, especially for the
	// "default" case of the drain statement (since as of this writing,
	// nothing in the rest of this file will ever exercise that).
	time.Sleep(time.Millisecond)

	s.cleanup()

	err := <-errors
	if err != ErrClosed {
		t.Fatal("Could somehow get a substream from a closing/closed stream")
	}
	// assert the two goroutines completed
	<-complete
	<-complete

	if !s.closed {
		t.Fatal("Stream did not close as expected.")
	}
}

func TestClosedStream(t *testing.T) {
	s, _, _ := getTestStream()
	s.Close()

	_, err := s.SubstreamToUser()
	if err != ErrClosed {
		t.Fatal("closed stream yielded substream")
	}
	_, err = s.SubstreamFromUser()
	if err != ErrClosed {
		t.Fatal("closed stream yielded substream")
	}
	_, err = s.Substream()
	if err != ErrClosed {
		t.Fatal("closed stream yielded substream")
	}
}

func TestDoubleCloseSubstream(t *testing.T) {
	s, _, _ := getTestStream()
	defer s.Close()

	ss, _ := s.SubstreamToUser()
	err := ss.Close()
	if err != nil {
		t.Fatal("Can't close substreams")
	}
	err = ss.Close()
	// also, no deadlocks
	if err == nil {
		t.Fatal("Can't properly double-close substreams")
	}

	// cover the case where the Stream has already closed a substream. This is
	// is hard to poke legitimately but can happen if the substream closes more
	// than once in quick enough succession.
	ss.closed = false
	ss.Close()
}

func TestFromUserReceiveWhenStreamClose(t *testing.T) {
	s, _, _ := getTestStream()

	ss, _ := s.SubstreamFromUser()
	go s.Close()

	msg, err := ss.Receive()
	if msg != nil || err != ErrClosed {
		t.Fatal("When stream closes, recieve-only substream doesn't notice properly.")
	}
	if !ss.closed {
		t.Fatal("When stream closes, receive-only substream isn't marked closed")
	}
}

func TestEmitWhenStreamClose(t *testing.T) {
	s, _, _ := getTestStream()

	ss, _ := s.SubstreamToUser()
	s.Close()

	err := ss.Send(1)
	if err != ErrClosed {
		t.Fatal("Did not get stream closed when sending the message")
	}
}

/*
func TestSubstreamDrain(t *testing.T) {
	ss := &substream{
		toUser:   make(chan EventToUser),
		fromUser: make(chan interface{}),
	}

	go func() {
		ss.fromUser <- ""
		close(ss.fromUser)
	}()

	err := ss.close()
	if err != ErrClosed {
		t.Fatal("Substream drain not working as expected")
	}
}
*/

func TestCoverage(t *testing.T) {
	stop{}.isStreamCommand()
	setExternalStream{nil, nil}.isStreamCommand()
	getSubstream{}.isStreamCommand()
	dopanic{123}.isStreamCommand()

	// test that we panic correctly
	c := make(chan struct{})
	logger := func(string, ...interface{}) {
		c <- struct{}{}
	}
	s, _, _ := getTestStream()
	s.logger = logger
	s.commands <- dopanic{123}
	<-c
}
