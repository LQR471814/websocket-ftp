package server

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"log"
	"math"
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
	files  []*TestFile
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
	log.Println("Transfer complete ->", t.Data, f)
	for _, r := range h.files {
		if strings.Contains(r.name, f.Name) {
			log.Println("File", f.Name, "has been transferred")
			r.recvdata = t.Output[f.ID()].(*bytes.Buffer).Bytes()
		}
	}
}

func (h TestHooks) OnAllTransfersComplete(t *Transfer) {
	log.Println("Transfer", t.ID, "is complete")
}

func AssertEqualBytes(name string, b1 []byte, b2 []byte, t *testing.T) {
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
				"\nFile %v Source: %v != Destination: %v",
			name, b1, b2,
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

	server := NewServer(ServerConfig{
		Handlers: TestHooks{files, choice},
		Verbose:  true,
	})

	go server.ServeWith(ln)

	return server, ln
}

const generatedFilename = "test_files/TEST_LARGE.bin"

func GenerateLargeFile() {
	f, err := os.Open(generatedFilename)
	if err != nil {
		f, err = os.Create(generatedFilename)
		if err != nil {
			log.Fatal(err)
		}

		buff := make([]byte, int(math.Pow(2, 27)))
		rand.Read(buff)

		_, err := f.Write(buff)
		if err != nil {
			log.Fatal(err)
		}
	}
	f.Close()
}

func TestGeneric(t *testing.T) {
	var err error
	os.Chdir("..")

	GenerateLargeFile()

	testFiles := []*TestFile{
		{name: "test_files/TEST_FILE1.txt"},
		{name: "test_files/TEST_FILE2.jpg"},
		{name: "test_files/TEST_FILE3.flp"},
		{name: generatedFilename},
	}

	for _, tf := range testFiles {
		tf.recvdata = make([]byte, 0)
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
	RunClient("ws://127.0.0.1:3000", testFiles[0:1])
	RunClient("ws://127.0.0.1:3000", testFiles[1:])
	server.Close()
	ln.Close()

	for _, tf := range testFiles {
		AssertEqualBytes(tf.name, tf.data, tf.recvdata, t)
	}
}
