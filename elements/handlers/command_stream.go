package handlers

import (
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/thejerf/sphyraena/request"
)

const (
	msgTerminate = "terminate"
)

// CommandSpecification allows you to specify a command to be run by the
// CommandResult stream.
//
// The *exec.Cmd is the command that will be executed for the stream.
// The Stdout and Stderr values will be overwritten by the handler, but
// everything will be used as you specify.
//
// StdoutFilter and StderrFilter allow you to override the writers used
// to send output to the user. The passed-in io.Writer will write out the
// CommandOutput message with the given bytes. The return value of this
// function will be used as the writer on the command instead. This can
// be used to filter the output, transform it, implement some form of
// Nagle's algorithm on the output, etc. If the function is nil or the
// return value of the function is nil, the default writer, which will
// simply output the value of the command to the stream, will be used.
//
// Once passed to one of these functions, the functions should be assumed
// to own the Command.
type CommandSpecification struct {
	Command *exec.Cmd
	// StdinFilter  func(io.Reader) io.Reader // not used yet
	StdoutFilter func(io.Writer) io.Writer
	StderrFilter func(io.Writer) io.Writer

	// A log.Printf-like function for logging. If nil, will use log.Printf.
	Logger func(string, ...interface{})
}

func (cs CommandSpecification) Log(msg string, params ...interface{}) {
	if cs.Logger == nil {
		log.Printf(msg, params...)
	} else {
		cs.Logger(msg, params...)
	}
}

// CmdOutMessage will be sent when the command emits something on either
// standard out or standard error.
type CmdOutMessage struct {
	Type    string `json:"type"` // "out" or "err", indicating stdout or stderr
	Content string `json:"content"`
}

// CmdError will be emitted when the execution of the command itself
// resulted in an error.
type CmdError struct {
	Error string `json:"error"`
}

// CmdExit will be emitted when the command has exited, and contains the
// error code returned by it.
type CmdExited struct {
	Type     string `json:"type"`
	ExitCode int    `json:"exit_code"`
}

// CmdTerminate is a message sent back to the stream that tells the server
// to terminate the stream.
type CmdTerminate struct{}

// CommandResult is a handler that can be used to stream results of
// commands out to the user. It is unidirectional and only sends results.
//
// The intended usage of this is to set up the CommandSpecification in your
// own handler, then pass control off to this handler. Error handling is
// left up to the original caller, which is why an error is returned.
func CommandResult(
	spec CommandSpecification,
	req *request.Request,
) error {
	s, err := req.SubstreamToUser()
	if err != nil {
		return err
	}
	stdoutC := make(chan CmdOutMessage)
	stderrC := make(chan CmdOutMessage)
	defer func() {
		_ = s.Close()

		// Drain the channels if we exit without them getting drained. This
		// should be constructed so that if we are exiting, the command is
		// terminated already, so that the sources of these channels are
		// already guaranteed to produce no more messages.
	DRAIN1LOOP:
		for {
			select {
			case m := <-stdoutC:
				fmt.Println(m)
			default:
				break DRAIN1LOOP
			}
		}
	DRAIN2LOOP:
		for {
			select {
			case <-stderrC:
			default:
				break DRAIN2LOOP
			}
		}
	}()

	stdout := io.Writer(cmdOutWriter{"out", stdoutC})
	stderr := io.Writer(cmdOutWriter{"err", stderrC})

	if spec.StdoutFilter != nil {
		replaceStdout := spec.StdoutFilter(stdout)
		if replaceStdout != nil {
			stdout = replaceStdout
		}
	}
	if spec.StderrFilter != nil {
		replaceStderr := spec.StderrFilter(stderr)
		if replaceStderr != nil {
			stderr = replaceStderr
		}
	}

	spec.Command.Stdout = stdout
	spec.Command.Stderr = stderr
	spec.Command.Stdin = nil

	cmdExecStatus := make(chan error)
	go func() {
		// it is not clear what to do if this panics... it really shouldn't
		// barring a serious bug in the command execution support.
		cmdExecStatus <- spec.Command.Start()
		cmdExecStatus <- spec.Command.Wait()
	}()
	startError := <-cmdExecStatus
	if startError != nil {
		s.Send(CmdError{startError.Error()})
		return startError
	}

	incoming, eventsToUser := s.RawChans()

	caughtExitCode := false

	// Looks like we have successfully processed the request
	req.StreamResponse(request.StreamRequestResult{
		SubstreamID: s.SubstreamID(),
	})

	// command is now guaranteed to have been started, successfully at
	// least according to the OS.
	defer func() {
		spec.Command.Process.Kill()

		// If we did not yet catch the exit code, we need to drain it.
		if !caughtExitCode {
			go func() {
				// there's a small chance this leaks, if the top-level
				// process literally never exits.
				// FIXME: Handle this case with a timeout or something.
				<-cmdExecStatus
			}()
		}
	}()

	// And now we enter a message pump, managing the messages going back
	// and forth between all these bits and pieces.
	for {
		fmt.Println("Cmd loop")
		select {
		case msg, ok := <-incoming:
			if !ok {
				fmt.Println(msg)
				// remote stream has closed, time to terminate everything.
				return nil
			}

			switch msg.Type {
			case msgTerminate:
				fmt.Println("Requested termination of command")
				return nil
			default:
				spec.Log("unknown message received: %#v", msg)
			}

		// if we get something from the command on standard out or
		// standard error, send it out the stream to the user.
		case outmsg := <-stdoutC:
			// we deliberately want to backpressure here, I think. FIXME:
			// but if the stream closes, do I lose this channel?
			eventsToUser <- s.Message(outmsg)
		case outmsg := <-stderrC:
			eventsToUser <- s.Message(outmsg)
		case <-cmdExecStatus:
			eventsToUser <- s.Message(
				// FIXME: Figure out how to get the exit code
				// FIXME: handle if it's not an exit code!
				CmdExited{ExitCode: 0, Type: "exit"},
			)
			caughtExitCode = true
		}
	}
}

type cmdOutWriter struct {
	ty      string
	cmdOutC chan CmdOutMessage
}

func (cow cmdOutWriter) Write(b []byte) (int, error) {
	cow.cmdOutC <- CmdOutMessage{
		Type:    cow.ty,
		Content: string(b),
	}
	return len(b), nil
}
