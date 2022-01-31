package server

import (
	"net"
	"net/http"

	utils "github.com/LQR471814/go-utils"
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
	OnTransferRequest(*Transfer) chan bool
	OnTransferUpdate(*Transfer)
	OnTransferComplete(*Transfer, File)
	OnAllTransfersComplete(*Transfer)
}

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(*http.Request) bool { return true }, //? Allow cross-origin requests
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WSFTPServer struct {
	Transfers map[uint64]*Transfer
	Config    ServerConfig

	ids    utils.IDStore
	server http.Server
}

type ServerConfig struct {
	Handlers ServerHooks
	Out      chan FileOut
	Verbose  bool
}

func NewDefaultServer() *WSFTPServer {
	return &WSFTPServer{
		Transfers: map[uint64]*Transfer{},
		Config:    ServerConfig{},
	}
}

func NewServer(config ServerConfig) *WSFTPServer {
	return &WSFTPServer{
		Transfers: map[uint64]*Transfer{},
		Config:    config,
	}
}

func initHTTP(s *WSFTPServer) http.Server {
	return http.Server{
		Handler: http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				Handler(s, w, r)
			},
		),
	}
}

func (s *WSFTPServer) Serve() {
	s.server = initHTTP(s)
	s.server.ListenAndServe()
}

func (s *WSFTPServer) ServeWith(listener net.Listener) {
	s.server = initHTTP(s)
	s.server.Serve(listener)
}

func (s *WSFTPServer) Close() {
	s.server.Close()
}
