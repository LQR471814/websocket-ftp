package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
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

func outputWriter(s *WSFTPServer, f File, t *Transfer) {
	_, ok := t.Output[f.ID()]
	if !ok {
		t.Output[f.ID()] = bytes.NewBuffer(nil)
	}

	var writtenBytes int64 = 0
	w := bufio.NewWriterSize(t.Output[f.ID()], 1024*1024*50) //? Buffsize = 50 mB

	for data := range t.dataChan {
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
		ID:     s.ids.Fetch(),
		State:  TransferState{Number: INITIAL},
		Output: make(map[string]io.Writer),
		conn:   conn,
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
		t.dataChan = make(chan []byte)
		f := t.Data.Files[t.State.CurrentFile]
		go outputWriter(s, f, t)
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
