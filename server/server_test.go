package server

import (
	"bytes"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type TestFile struct {
	name     string
	data     []byte
	recvdata []byte
}

type TestHooks struct {
	choice bool
}

func (h TestHooks) OnTransferRequest(t *Transfer) chan bool {
	log.Println("Got requests", t.Data.Files)
	c := make(chan bool, 1)
	c <- h.choice
	return c
}

func (h TestHooks) OnTransferUpdate(t *Transfer) {
	log.Println(
		"Progress",
		t.State.Received,
		"/",
		t.Data.Files[t.State.CurrentFile].Size,
	)
}

func (h TestHooks) OnTransferComplete(t *Transfer, f File) {
	log.Println("File", f.Name, "has been transferred")
}

func (h TestHooks) OnAllTransfersComplete(t *Transfer) {
	log.Println("Transfer", t.ID, "is complete")
}

func AssertEqualBytes(b1 []byte, b2 []byte, t *testing.T) {
	if !bytes.Equal(b1, b2) {
		const cutoff = 16

		if len(b1) > cutoff {
			b1 = b1[:cutoff]
		}

		if len(b2) > cutoff {
			b2 = b2[:cutoff]
		}

		t.Errorf(
			"Transferred result is not equivalent to source!"+
				"\nSource: %v != Destination: %v",
			b1, b2,
		)
	}
}

func RunClient(ip string, files []*TestFile) {
	argFiles := []string{}
	for _, f := range files {
		argFiles = append(argFiles, "--files")
		argFiles = append(argFiles, f.name)
	}

	cmd := exec.Cmd{
		Path: "node",
		Args: append([]string{
			"node", "./test_client/index.js",
			"--url", "ws://127.0.0.1:3000",
		}, argFiles...),
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Stderr = cmd.Stdout

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	for {
		buff := make([]byte, 1024)
		n, err := stdout.Read(buff)
		if n > 0 {
			log.Print("[Client] ", string(buff[:n]))
		}
		if err != nil {
			break
		}
	}
}

func RunServer(ip string, files []*TestFile, choice bool) (*WSFTPServer, net.Listener) {
	ln, err := net.Listen("tcp", ip)
	if err != nil {
		log.Fatal(err)
	}

	out := make(chan FileOut)

	server := NewServer(ServerConfig{
		Handlers: TestHooks{choice},
		Out:      out,
		Verbose:  true,
	})

	go server.ServeWith(ln)
	go func() {
		for o := range out {
			for _, f := range files {
				if strings.Contains(f.name, o.File.Name) {
					f.recvdata = o.Buffer
				}
			}
		}
	}()

	return server, ln
}

func TestGeneric(t *testing.T) {
	var err error

	testFiles := []*TestFile{
		{name: "test_files/TEST_FILE1.txt"},
		{name: "test_files/TEST_FILE2.jpg"},
		{name: "test_files/TEST_FILE3.flp"},
	}

	os.Chdir("..")

	for _, tf := range testFiles {
		tf.data, err = ioutil.ReadFile(tf.name)
		if err != nil {
			log.Fatal(err)
		}
	}

	server, ln := RunServer("127.0.0.1:3000", testFiles, false)
	RunClient("ws://127.0.0.1:3000", testFiles)
	server.Close()
	ln.Close()

	server, ln = RunServer("127.0.0.1:3000", testFiles, true)
	RunClient("ws://127.0.0.1:3000", testFiles)
	server.Close()
	ln.Close()

	for _, tf := range testFiles {
		AssertEqualBytes(tf.data, tf.recvdata, t)
	}
}
