package tpt

import (
	"log"
	"net"
)

type Transport struct {
	port     int
	connChan chan net.Conn
	shutdown chan bool
}

func NewTransport(port int) *Transport {
	t := &Transport{
		port,
		make(chan net.Conn),
		make(chan bool),
	}

	go t.tcpListen()

	return t
}

func (t *Transport) tcpListen() {
	l, err := net.ListenTCP("tcp", &net.TCPAddr{nil, t.port, ""})
	defer l.Close()

	if err != nil {
		log.Fatalf("Failed to listen on port %d", t.port)
	}

	log.Printf("Listening on: %d", t.port)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("Failed to accept connection: %v", err)
		}

		t.connChan <- conn

	}
}

func (t *Transport) ConnCh() chan net.Conn {
	return t.connChan
}
