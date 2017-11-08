// Contains the implementation of a LSP client.

package lsp

import (
	"errors"
	"github.com/cmu440/lspnet"
	"encoding/json"
	"log"
	//"fmt"
	"fmt"
)

const(
	INIT_SEQ_NUM = 0
)

type client struct {
	conn        *lspnet.UDPConn
	serverAddr  *lspnet.UDPAddr
	connID      int
	nextSeqNum  int
	chanConnect chan bool
	chanRead    chan Message
	chanWrite   chan Message
	chanOut     chan Message
	chanIn      chan Message
	win         *slidingWindow
	params      *Params
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
		conn:        conn,
		serverAddr:  addr,
		chanConnect: make(chan bool),
		chanRead:    make(chan Message),
		chanWrite:   make(chan Message),
		chanOut:     make(chan Message),
		chanIn:      make(chan Message),
		params:      params,
	}
	go c.read()
	go c.write()
	go c.stateMachine()

	msg := NewConnect()
	c.chanOut <- *msg
	if <-c.chanConnect {
		fmt.Printf("[Connect]Connected to server: %v\n",hostport)
		return c, nil
	}
	return nil, errors.New("Connection Closed!")
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

func (c *client) stateMachine() {
	for {
		select {
		case msg := <-c.chanIn:
			switch msg.Type {
			case MsgAck:
				if msg.SeqNum == INIT_SEQ_NUM {
					c.connID = msg.ConnID
					c.nextSeqNum = INIT_SEQ_NUM + 1
					w := NewWindow(c.chanOut, c.params.WindowSize, c.nextSeqNum)
					c.win = w
					c.chanConnect <- true
				} else {
					c.win.NewACK(msg)
				}
			case MsgData:
				c.chanRead <- msg
				ack := NewAck(msg.ConnID, msg.SeqNum)
				c.chanOut <- *ack
			}
		case msg := <-c.chanWrite:
			c.win.NewMessage(msg)
		}
	}
}

func (c *client) read() {
	msg := Message{}
	buf := make([]byte, 1024)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		json.Unmarshal(buf[:n], &msg)
		fmt.Printf("[Read]Read from server: %v\n",msg)
		c.chanIn <- msg
	}
}

func (c *client) write() {
	for msg := range c.chanOut {
		buf, err := json.Marshal(msg)
		if err != nil {
			log.Fatal(err)
		}
		c.conn.Write(buf)
		fmt.Printf("[Write]Write to server: %v\n",msg)
	}
}
