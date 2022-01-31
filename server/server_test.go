package wsftp

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

type TestHooks struct{}

func (h TestHooks) OnTransferRequest(metadata TransferMetadata) chan bool {
	log.Println("Got requests", metadata)
	c := make(chan bool, 1)
	c <- true
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

func (h TestHooks) OnTransfersComplete(id string) {
	log.Println("Transfer", id, "is complete")
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

func SetupConfig() chan FileOut {
	out := make(chan FileOut)
	SetServerConfig(ServerConfig{
		Out:      out,
		Handlers: TestHooks{},
		Verbose:  true,
	})
	return out
}

func RunServer(ip string, files []*TestFile) {
	ln, err := net.Listen("tcp", ip)
	if err != nil {
		log.Fatal(err)
	}

	go ServeWith(ln)
	go func() {
		for o := range SetupConfig() {
			for _, f := range files {
				if strings.Contains(f.name, o.File.Name) {
					f.recvdata = o.Buffer
				}
			}
		}
	}()
}

func TestConfiguration(t *testing.T) {
	SetupConfig()
}

func TestDefaultServe(t *testing.T) {
	go Serve()
}

func TestServer(t *testing.T) {
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

	RunServer("127.0.0.1:3000", testFiles)

	log.Println("Building...")

	os.Chdir("client")
	exec.Command("npm", "run", "build").Run()
	os.Chdir("../test_client")
	exec.Command("npm", "run", "build").Run()
	os.Chdir("..")

	log.Println("Done")

	RunClient("ws://127.0.0.1:3000", testFiles)

	const cutoff = 16
	for _, tf := range testFiles {
		if !bytes.Equal(tf.data, tf.recvdata) {
			var b1 []byte
			var b2 []byte

			if len(tf.data) > cutoff {
				b1 = tf.data[:cutoff]
			} else {
				b1 = tf.data
			}

			if len(tf.recvdata) > cutoff {
				b2 = tf.recvdata[:cutoff]
			} else {
				b2 = tf.recvdata
			}

			t.Errorf(
				"Transferred result is not equivalent to source!"+
					"\nSource: %v != Destination: %v",
				b1, b2,
			)
		}
	}
}
