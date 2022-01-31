package server

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

func sendJSON(c *websocket.Conn, obj interface{}) {
	msg, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	c.WriteMessage(websocket.TextMessage, msg)
}

func outputWriter(s *WSFTPServer, f File, datachan chan []byte) {
	var output io.Writer
	if s.Config.Out == nil {
		f, err := os.Create(f.Name)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		output = f
	} else {
		fileoutput := &FileOut{File: f}
		defer func() { s.Config.Out <- *fileoutput }()
		output = fileoutput
	}

	var writtenBytes int64 = 0
	w := bufio.NewWriterSize(output, 1024*1024*50) //? Buffsize = 50 mB

	for data := range datachan {
		w.Write(data)
		writtenBytes += int64(len(data))
		if writtenBytes >= f.Size {
			if s.Config.Verbose {
				log.Printf("--> DONE: Wrote %v to output\n", f.Name)
			}
			w.Flush()
			return
		}
	}
}

func Handler(s *WSFTPServer, w http.ResponseWriter, r *http.Request) {
	var updateRatio = 24
	if s.Config.Verbose {
		updateRatio = 4
	}

	conn, err := upgrader.Upgrade(w, r, nil) //? Event: onpeerconnect
	if err != nil {
		panic(err)
	}

	transfer := &Transfer{
		Data: TransferMetadata{
			From: conn.RemoteAddr().(*net.TCPAddr).IP,
		},
		ID:    s.ids.Fetch(),
		State: TransferState{Number: INITIAL},
		conn:  conn,
	}

	s.Transfers[transfer.ID] = transfer
	eventHandler(s, transfer, peerConnect)

	var updateNext int64 = 0

	for {
		msgType, contents, err := conn.ReadMessage()
		if err != nil {
			if strings.Contains(err.Error(), "close") {
				return
			}
			panic(err)
		}

		switch msgType {
		case websocket.TextMessage:
			//? This will 100% come back to bite me later but this should be fine for now
			reqs := &FileRequests{}
			json.Unmarshal(contents, reqs)
			transfer.Data.Files = reqs.Files

			eventHandler(s, transfer, recvRequests)
		case websocket.BinaryMessage:
			f := transfer.Data.Files[transfer.State.CurrentFile]

			updateOffset := f.Size / int64(updateRatio)

			transfer.dataChan <- contents
			transfer.State.Received += int64(len(contents))

			if transfer.State.Received >= updateNext {
				if s.Config.Handlers != nil {
					s.Config.Handlers.OnTransferUpdate(transfer)
				}
				updateNext += updateOffset
			}

			if transfer.State.Received >= f.Size {
				eventHandler(s, transfer, recvDone)
				updateNext = 0
			}
		}
	}
}

func eventHandler(s *WSFTPServer, t *Transfer, event Event) {
	cell, ok := EventStateMatrix[event][t.State.Number]
	if s.Config.Verbose {
		log.Println("Event", event, "State", t.State, cell)
	}

	if !ok {
		panic("Invalid FileTransfer state")
	}

	t.State.Number = cell.NewState
	for _, action := range cell.Actions {
		actionHandler(s, t, action)
	}
}

func actionHandler(s *WSFTPServer, t *Transfer, action Action) {
	switch action {
	case DisplayFileRequests:
		var accept = true
		if s.Config.Handlers != nil {
			accept = <-s.Config.Handlers.OnTransferRequest(t)
		}
		if accept {
			eventHandler(s, t, userAccept)
			return
		}
		eventHandler(s, t, userDeny)
	case IncrementFileIndex:
		s.Config.Handlers.OnTransferComplete(t, t.Data.Files[t.State.CurrentFile])
		t.State.CurrentFile += 1
		t.State.Received = 0
	case SendStartSignal:
		sendJSON(t.conn, Signal{Type: "start"})
	case SendExitSignal:
		sendJSON(t.conn, Signal{Type: "exit"})
	case SendFinishedSignal:
		sendJSON(t.conn, Signal{Type: "complete"})
	case StartFileWriter:
		f := t.Data.Files[t.State.CurrentFile]
		t.dataChan = make(chan []byte)
		go outputWriter(s, f, t.dataChan)
	case StopFileWriter:
		close(t.dataChan)
	case RecvDoneHandler:
		if t.State.CurrentFile >= len(t.Data.Files) {
			s.Config.Handlers.OnAllTransfersComplete(t)
			return
		}
		actionHandler(s, t, StartFileWriter)
		actionHandler(s, t, SendStartSignal)
	}
}
