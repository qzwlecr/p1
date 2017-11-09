// Contains the implementation of a LSP client.

package lsp

import (
	"errors"
	"github.com/cmu440/lspnet"
	"encoding/json"
	"log"
	"time"
)

type client struct {
	conn         *lspnet.UDPConn
	serverAddr   *lspnet.UDPAddr
	connID       int
	nextSeqNum   int
	chanConnect  chan bool
	chanRead     chan Message
	chanWrite    chan Message
	chanOut      chan Message
	chanIn       chan Message
	win          *slidingWindow_
	params       *Params
	chanRstEpoch chan bool
	chanCntEpoch chan int
}

// NewClient creates, initiates, and returns a new client. This function
// should return after a connection with the server has been established
// (i.e., the client has received an Ack message from the server in response
// to its connection request), and should return a non-nil error if a
// connection could not be made (i.e., if after K epochs, the client still
// hasn't received an Ack message from the server in response to its K
// connection requests).
//
// hostport is a colon-separated string identifying the server's host address
// and port number (i.e., "localhost:9999").
func NewClient(hostport string, params *Params) (Client, error) {
	addr, err := lspnet.ResolveUDPAddr("udp", hostport)
	if err != nil {
		return nil, err
	}
	conn, err := lspnet.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}

	c := &client{
		conn:         conn,
		serverAddr:   addr,
		chanConnect:  make(chan bool),
		chanRead:     make(chan Message),
		chanWrite:    make(chan Message),
		chanOut:      make(chan Message),
		chanIn:       make(chan Message),
		params:       params,
		chanRstEpoch: make(chan bool),
		chanCntEpoch: make(chan int),
	}

	go c.read()
	go c.write()
	go c.stateMachine()
	go c.epoch()

	return c.connect()

}

func (c *client) ConnID() int {
	return c.connID
}

func (c *client) Read() ([]byte, error) {
	select {
	case msg, open := <-c.chanRead:
		if open {
			return msg.Payload, nil
		} else {
			return nil, errors.New("Connection Closed!")
		}
	}
}

func (c *client) Write(payload []byte) error {
	msg := NewData(c.connID, c.nextSeqNum, len(payload), payload)
	c.nextSeqNum++
	c.chanWrite <- *msg
	return nil
}

func (c *client) Close() error {
	return errors.New("not yet implemented")
}

func (c *client) connect() (Client, error) {
	msg := NewConnect()
	c.chanOut <- *msg
	if <-c.chanConnect {
		log.Printf("[C][Connect]Client %v: Connected to server: %v\n", c.connID, c.serverAddr)
		return c, nil
	}
	return nil, errors.New("Connection Closed!")
}

func (c *client) read() {
	msg := new(Message)
	buf := make([]byte, BufferSize)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		json.Unmarshal(buf[:n], msg)
		log.Printf("[C][Read]Client %v: Read from server: %v\n", c.connID, *msg)
		c.chanRstEpoch <- true
		c.chanIn <- *msg
	}
}

func (c *client) write() {
	for msg := range c.chanOut {
		buf, err := json.Marshal(msg)
		if err != nil {
			log.Fatal(err)
		}
		c.conn.Write(buf)
		log.Printf("[C][Write]Client %v: Write to server: %v\n", c.connID, msg)
	}
}

func (c *client) epoch() {
	epochTime := time.Duration(c.params.EpochMillis) * time.Millisecond
	t := time.Tick(epochTime)
	cnt := 0
	for {
		select {
		case rst := <-c.chanRstEpoch:
			if rst {
				log.Printf("[C][Epoch]Client %v: Reset Epoch\n", c.connID)
				cnt = 0
				t = time.Tick(epochTime)
				//reset time ticker
			} else {
				log.Printf("[C][Epoch]Client %v: Close Epoch\n", c.connID)
				return
			}
		case <-t:
			cnt++
			c.win.chanOp <- RESEND
			log.Printf("[C][Epoch]Client %v: Cnt = %v\n", c.connID, cnt)
			c.chanCntEpoch <- cnt
		}
	}
}

func (c *client) stateMachine() {
	for {
		select {
		case msg := <-c.chanIn:
			switch msg.Type {
			case MsgAck:
				if msg.SeqNum == InitSeqNum {
					c.connID = msg.ConnID
					c.nextSeqNum = InitSeqNum + 1
					w := NewWindow_(c.chanOut, c.params.WindowSize, c.nextSeqNum)
					c.win = w
					c.chanConnect <- true
				} else {
					c.win.chanOp <- ACK
					c.win.chanIn <- msg
				}
			case MsgData:
				c.chanRead <- msg
				c.chanOut <- *NewAck(msg.ConnID, msg.SeqNum)
			}
		case msg := <-c.chanWrite:
			c.win.chanOp <- MESSAGE
			c.win.chanIn <- msg
		case cnt := <-c.chanCntEpoch:
			log.Printf("[C][Epoch]Client %v: Resend\n", c.connID)
			if cnt >= c.params.EpochLimit {
				c.chanRstEpoch <- false
			}
		}
	}
}
