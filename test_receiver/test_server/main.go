package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(*http.Request) bool { return true }, //? Allow cross-origin requests
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	var client *websocket.Conn
	var receiver *websocket.Conn

	server := http.Server{
		Handler: http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil) //? Event: onpeerconnect
				if err != nil {
					panic(err)
				}
				log.Println(r.URL.Path)
				if r.URL.Path == "/client" {
					if receiver == nil {
						log.Fatal("Receiver has not connected yet")
					}
					client = conn
					go func() {
						for {
							t, msg, err := client.ReadMessage()
							if err != nil {
								return
							}
							receiver.WriteMessage(t, msg)
						}
					}()
				} else if r.URL.Path == "/receive" {
					receiver = conn
					go func() {
						for {
							t, msg, err := receiver.ReadMessage()
							if err != nil {
								return
							}
							client.WriteMessage(t, msg)
						}
					}()
				}
			},
		),
	}

	listener, err := net.Listen("tcp", "localhost:3000")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Listening on port 3000")
	go func() { server.Serve(listener) }()

	args := []string{
		"--files", "test_files/TEST_FILE1.txt",
		"test_files/TEST_FILE2.jpg",
		"test_files/TEST_FILE3.flp",
		"test_files/TEST_LARGE.bin",
	}

	os.Chdir("../..")
	receiveProcess := exec.Command(
		"node", append([]string{
			"test_receiver/index.js",
			"--url", "ws://localhost:3000/receive",
		}, args...)...,
	)

	receiveStdout, err := receiveProcess.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	receiveProcess.Stderr = receiveProcess.Stdout

	err = receiveProcess.Start()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			buff := make([]byte, 1024)
			n, err := receiveStdout.Read(buff)
			if n > 0 {
				log.Print("[Receiver] ", string(buff[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	time.Sleep(time.Second * 1)

	clientProcess := exec.Command(
		"node", append([]string{
			"test_client/index.js",
			"--url", "ws://localhost:3000/client",
		}, args...)...,
	)

	clientStdout, err := clientProcess.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	clientProcess.Stderr = clientProcess.Stdout

	err = clientProcess.Start()
	if err != nil {
		log.Fatal(err)
	}

	for {
		buff := make([]byte, 1024)
		n, err := clientStdout.Read(buff)
		if n > 0 {
			log.Print("[Client] ", string(buff[:n]))
		}
		if err != nil {
			break
		}
	}
}
