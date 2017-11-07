// Contains the implementation of a LSP server.

package lsp

import (
	"errors"
	"github.com/cmu440/lspnet"
	"strconv"
	"encoding/json"
)

type clientInfo struct {
	connId     int
	nextSeqNum int
	clientAddr *lspnet.UDPAddr
	chanIn     chan Message
	chanOut    chan Message
}
type server struct {
	listener        *lspnet.UDPConn
	clients         map[int]*clientInfo
	chanNewConn     chan *lspnet.UDPAddr
	chanRead        chan Message
	chanWrite       chan Message
	chanCloseConn   chan int
	chanCloseServer chan struct{}
	chanConnId      chan int
}

// NewServer creates, initiates, and returns a new server. This function should
// NOT block. Instead, it should spawn one or more goroutines (to handle things
// like accepting incoming client connections, triggering epoch events at
// fixed intervals, synchronizing events using a for-select loop like you saw in
// project 0, etc.) and immediately return. It should return a non-nil error if
// there was an error resolving or listening on the specified port number.
func NewServer(port int, params *Params) (Server, error) {
	addr, err := lspnet.ResolveUDPAddr("udp", lspnet.JoinHostPort("localhost", strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}

	conn, err := lspnet.ListenUDP("udp", addr)
	s := &server{
		listener:        conn,
		clients:         make(map[int]*clientInfo),
		chanNewConn:     make(chan *lspnet.UDPAddr),
		chanRead:        make(chan Message),
		chanWrite:       make(chan Message),
		chanCloseConn:   make(chan int),
		chanCloseServer: make(chan struct{}),
		chanConnId: func() chan int {
			gen := make(chan int)
			count := 0
			go func() {
				for {
					gen <- count
					count ++
				}
			}()
			return gen
		}(),
	}
	go s.read()
	go s.serverStateMachine()
	return s, nil
}

func (s *server) Read() (int, []byte, error) {
	select {
	case msg, ok := <-s.chanRead:
		if ok {
			return msg.ConnID, msg.Payload, nil
		}
	} // Blocks indefinitely.
	return -1, nil, errors.New("not yet implemented")
}

func (s *server) Write(connID int, payload []byte) error {
	msg := NewData(connID, -1, len(payload), payload)
	s.chanWrite <- *msg
	return nil
}

func (s *server) CloseConn(connID int) error {
	s.chanCloseConn <- connID
	return nil
}

func (s *server) Close() error {
	var zero struct{}
	s.chanCloseServer <- zero
	return errors.New("not yet implemented")
}

func (s *server) serverStateMachine() error {
	for {
		select {
		case cAddr := <-s.chanNewConn:
			c := &clientInfo{
				connId:     <-s.chanConnId,
				nextSeqNum: 1,
				clientAddr: cAddr,
				chanIn:     make(chan Message),
				chanOut:    make(chan Message),
			}
			s.clients[c.connId] = c
			go s.clientStateMachine(c)
			c.chanOut <- *NewAck(c.connId, 0)
		case msg := <-s.chanWrite:
			if c, ok := s.clients[msg.ConnID]; ok {
				if msg.SeqNum == -1 {
					msg.SeqNum = c.nextSeqNum
					c.nextSeqNum ++
				}
				c.chanOut <- msg
			}
		case connId := <-s.chanCloseConn:
			if _, ok := s.clients[connId]; ok {
				c := s.clients[connId]
				close(c.chanIn)
				close(c.chanOut)
				delete(s.clients, connId)
			}
		}
	}
}

func (s *server) clientStateMachine(c *clientInfo) error {
	for {
		select {
		case msg := <-c.chanIn:
			switch msg.Type {
			case MsgData:
				s.chanRead <- msg
				buf, _ := json.Marshal(NewAck(msg.ConnID, msg.SeqNum))
				s.listener.WriteToUDP(buf, c.clientAddr)
			}
		case msg := <-c.chanOut:
			buf, _ := json.Marshal(msg)
			s.listener.WriteToUDP(buf, c.clientAddr)
		}
	}
}

func (s *server) read() {
	buf := make([]byte, 1024)
	msg := new(Message)
	for {
		n, cAddr, err := s.listener.ReadFromUDP(buf)
		if err != nil {
			return
		}
		json.Unmarshal(buf[:n], msg)
		switch msg.Type {
		case MsgConnect:
			s.chanNewConn <- cAddr
		default:
			if c, ok := s.clients[msg.ConnID]; ok {
				c.chanIn <- *msg
			}
		}
	}
}
