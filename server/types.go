package server

import (
	"net"

	"github.com/gorilla/websocket"
)

type Signal struct {
	Type string
}

type FileRequests struct {
	Type  string
	Files []File
}

type File struct {
	Name string
	Size int64
	Type string
}

type TransferMetadata struct {
	From  net.IP
	Files []File
}

type TransferState struct {
	Number      StateID
	CurrentFile int
	Received    int64
}

type Transfer struct {
	Data  TransferMetadata
	State TransferState
	ID    uint64

	conn     *websocket.Conn
	dataChan chan []byte
}

//? Note: WriteChunk / recvfilecontents is not present here
//?  because it would make things unnecessarily complex and inefficient

//?  The logic is still implemented, it's just not through the handlers

type StateID int

const (
	INITIAL StateID = iota
	LISTENING_FOR_FILE_REQUESTS
	WAITING_FOR_USER_CONFIRMATION
	RECEIVING
)

type Event int

const (
	peerConnect Event = iota
	recvRequests
	userAccept
	userDeny
	recvDone
)

type Action int

const (
	DisplayFileRequests Action = iota
	IncrementFileIndex
	SendClientAllow

	SendStartSignal
	SendExitSignal
	SendFinishedSignal

	StartFileWriter
	StopFileWriter

	RecvDoneHandler
)

type StateMatrix map[Event]map[StateID]struct {
	Actions  []Action
	NewState StateID
}

var EventStateMatrix = StateMatrix{
	peerConnect: {
		INITIAL: {
			Actions:  []Action{},
			NewState: LISTENING_FOR_FILE_REQUESTS,
		},
	},
	recvRequests: {
		LISTENING_FOR_FILE_REQUESTS: {
			Actions:  []Action{DisplayFileRequests},
			NewState: WAITING_FOR_USER_CONFIRMATION,
		},
	},
	userAccept: {
		WAITING_FOR_USER_CONFIRMATION: {
			Actions:  []Action{StartFileWriter, SendStartSignal},
			NewState: RECEIVING,
		},
	},
	userDeny: {
		WAITING_FOR_USER_CONFIRMATION: {
			Actions:  []Action{SendExitSignal},
			NewState: INITIAL,
		},
	},
	recvDone: {
		RECEIVING: {
			Actions: []Action{
				StopFileWriter,
				IncrementFileIndex,
				SendFinishedSignal,
				RecvDoneHandler,
			},
			NewState: RECEIVING,
		},
	},
}
