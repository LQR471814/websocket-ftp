package server

import "testing"

func TestHTTPInit(t *testing.T) {
	initHTTP(NewDefaultServer())
}

func TestDefaultServe(t *testing.T) {
	go NewDefaultServer().Serve()
}
