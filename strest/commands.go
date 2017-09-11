package strest

type streamCommand interface {
	isStreamCommand()
}

type stop struct{}

func (s stop) isStreamCommand() {}

type setExternalStream struct {
	toUser   chan EventToUser
	fromUser chan EventFromUser
}

func (ses setExternalStream) isStreamCommand() {}

type unsetExternalStream struct {
	toUser   chan EventToUser
	fromUser chan EventFromUser
}

func (ues unsetExternalStream) isStreamCommand() {}

type getSubstream struct {
	canReceive bool
	ss         chan substreamret
}

func (gs getSubstream) isStreamCommand() {}

type dopanic struct {
	panicval interface{}
}

func (d dopanic) isStreamCommand() {}
