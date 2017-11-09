// Contains the implementation of a LSP server.

package lsp

import (
	"errors"
	"github.com/cmu440/lspnet"
	"strconv"
	"encoding/json"
	"time"
	"log"
)

type clientInfo struct {
	connId       int
	nextSeqNum   int
	clientAddr   *lspnet.UDPAddr
	chanIn       chan Message
	chanOut      chan Message
	chanRstEpoch chan bool
	chanCntEpoch chan int
	window       *slidingWindow
	sortedMessage *sorter

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
	params          *Params
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
		params: params,
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
	msg := NewData(connID, 0, len(payload), payload)
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
				connId:       <-s.chanConnId,
				nextSeqNum:   InitSeqNum + 1,
				clientAddr:   cAddr,
				chanIn:       make(chan Message),
				chanOut:      make(chan Message),
				chanCntEpoch: make(chan int),
				chanRstEpoch: make(chan bool),
			}
			s.clients[c.connId] = c
			w := NewWindow(c.chanOut, s.params.WindowSize, c.nextSeqNum)
			c.window = w
			st := NewSorter(s.chanRead, c.nextSeqNum)
			c.sortedMessage = st
			go s.epoch(c)
			go s.write(c)
			go s.clientStateMachine(c)
			c.chanOut <- *NewAck(c.connId, 0)
		case msg := <-s.chanWrite:
			if c, ok := s.clients[msg.ConnID]; ok {
				if msg.Type == MsgData {
					msg.SeqNum = c.nextSeqNum
					c.nextSeqNum ++
					c.window.chanOp <- MESSAGE
					c.window.chanIn <- msg
				}
			}
		case connId := <-s.chanCloseConn:
			if c, ok := s.clients[connId]; ok {
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
				c.sortedMessage.Add(msg)
				c.chanOut <- *NewAck(msg.ConnID, msg.SeqNum)
			case MsgAck:
				c.window.chanOp <- ACK
				c.window.chanIn <- msg
			}
		case cnt := <-c.chanCntEpoch:
			if cnt >= s.params.EpochLimit {
				c.chanRstEpoch <- false
			}
		}
	}
}

func (s *server) read() {
	buf := make([]byte, BufferSize)
	msg := new(Message)
	for {
		n, cAddr, err := s.listener.ReadFromUDP(buf)
		if err != nil {
			return
		}
		json.Unmarshal(buf[:n], msg)
		log.Printf("[S][Read]Server Reading:%v\n", *msg)
		switch msg.Type {
		case MsgConnect:
			s.chanNewConn <- cAddr
		default:
			if c, ok := s.clients[msg.ConnID]; ok {
				c.chanIn <- *msg
				c.chanRstEpoch <- true
			}
		}
	}
}

func (s *server) write(c *clientInfo) {
	for {
		msg := <-c.chanOut
		buf, _ := json.Marshal(msg)
		log.Printf("[S][Write]Server Writing:%v\n", msg)
		s.listener.WriteToUDP(buf, c.clientAddr)
	}
}

func (s *server) epoch(c *clientInfo) {
	epochTime := time.Duration(s.params.EpochMillis) * time.Millisecond
	t := time.Tick(epochTime)
	cnt := 0
	for {
		select {
		case rst := <-c.chanRstEpoch:
			if rst {
				log.Printf("[S][Epoch]Client %v Reset Epoch\n", c.connId)
				t = time.Tick(epochTime)
				//reset time ticker
				cnt = 0
			} else {
				log.Printf("[S][Epoch]Client %v Close Epoch\n", c.connId)
				return
			}
		case <-t:
			cnt++
			c.window.chanOp <- RESEND
			log.Printf("[S][Epoch]Client %v: Cnt = %v\n", c.connId, cnt)
			c.chanCntEpoch <- cnt
		}
	}
}
