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

type FileOut struct {
	File   File
	Buffer []byte
}

func (out *FileOut) Write(b []byte) (n int, err error) {
	out.Buffer = append(out.Buffer, b...)
	return len(b), nil
}

type ServerHooks interface {
	OnTransferRequest(TransferMetadata) chan bool
	OnTransferUpdate(*Transfer)
	OnTransfersComplete(string)
}

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(*http.Request) bool { return true }, //? Allow cross-origin requests
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ServerConfig struct {
	Handlers ServerHooks
	Out      chan FileOut
	Verbose  bool
}

var config ServerConfig

func sendJSON(c *websocket.Conn, obj interface{}) {
	msg, err := json.Marshal(obj)
	if err != nil {
		log.Fatal(err)
	}

	c.WriteMessage(websocket.TextMessage, msg)
}

func outputWriter(f File, datachan chan []byte) {
	var output io.Writer
	if config.Out == nil {
		f, err := os.Create(f.Name)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		output = f
	} else {
		fileoutput := &FileOut{File: f}
		defer func() { config.Out <- *fileoutput }()
		output = fileoutput
	}

	var writtenBytes int64 = 0
	w := bufio.NewWriterSize(output, 1024*1024*50) //? Buffsize = 50 mB

	for data := range datachan {
		w.Write(data)
		writtenBytes += int64(len(data))
		if writtenBytes >= f.Size {
			if config.Verbose {
				log.Printf("--> DONE: Wrote %v to output\n", f.Name)
			}
			w.Flush()
			return
		}
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	var updateRatio = 24
	if config.Verbose {
		updateRatio = 4
	}

	conn, err := upgrader.Upgrade(w, r, nil) //? Event: onpeerconnect
	if err != nil {
		log.Fatal(err)
	}

	transfer := &Transfer{
		Data: TransferMetadata{
			From: conn.RemoteAddr().(*net.TCPAddr).IP,
		},
		State: TransferState{Number: INITIAL},
		conn:  conn,
	}

	eventHandler(transfer, peerConnect)

	var updateNext int64 = 0

	for {
		msgType, contents, err := conn.ReadMessage()
		if err != nil {
			if strings.Contains(err.Error(), "close") {
				return
			}
			log.Fatal(err)
		}

		switch msgType {
		case websocket.TextMessage:
			//? This will 100% come back to bite me later but this should be fine for now
			reqs := &FileRequests{}
			json.Unmarshal(contents, reqs)
			transfer.Data.Files = reqs.Files

			eventHandler(transfer, recvRequests)
		case websocket.BinaryMessage:
			f := transfer.Data.Files[transfer.State.CurrentFile]

			updateOffset := f.Size / int64(updateRatio)

			transfer.dataChan <- contents
			transfer.State.Received += int64(len(contents))

			if transfer.State.Received >= updateNext {
				if config.Handlers != nil {
					config.Handlers.OnTransferUpdate(transfer)
				}
				updateNext += updateOffset
			}

			if transfer.State.Received >= f.Size {
				eventHandler(transfer, recvDone)
				updateNext = 0
			}
		}
	}
}

func eventHandler(t *Transfer, event Event) {
	cell, ok := EventStateMatrix[event][t.State.Number]
	if config.Verbose {
		log.Println("Event", event, "State", t.State, cell)
	}

	if !ok {
		log.Fatal("FileTransfer: undefined state")
	}

	t.State.Number = cell.NewState
	for _, action := range cell.Actions {
		actionHandler(t, action)
	}
}

func actionHandler(t *Transfer, action Action) {
	switch action {
	case DisplayFileRequests:
		var accept = true
		if config.Handlers != nil {
			accept = <-config.Handlers.OnTransferRequest(t.Data)
		}
		if accept {
			eventHandler(t, userAccept)
			return
		}
		eventHandler(t, userDeny)
	case IncrementFileIndex:
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
		go outputWriter(f, t.dataChan)
	case StopFileWriter:
		close(t.dataChan)
	case RecvDoneHandler:
		if t.State.CurrentFile >= len(t.Data.Files) {
			return
		}
		actionHandler(t, StartFileWriter)
		actionHandler(t, SendStartSignal)
	}
}

func SetServerConfig(cfg ServerConfig) {
	config = cfg
}

func Serve() {
	server := http.Server{Handler: http.HandlerFunc(Handler)}
	server.ListenAndServe()
}

func ServeWith(listener net.Listener) {
	server := http.Server{Handler: http.HandlerFunc(Handler)}
	server.Serve(listener)
}
