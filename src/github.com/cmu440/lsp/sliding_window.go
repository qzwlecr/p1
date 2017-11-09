package lsp

import "log"

const (
	MESSAGE = iota
	ACK
	RESEND
)

type SlidingWindow interface {
	// Give a sending Message to sliding window
	NewMessage(msg Message)
	// Give a ACK to sliding window
	NewACK(msg Message)
}

type slidingWindow struct {
	window  []windowRecord
	chanOp  chan int
	chanIn  chan Message
	chanOut chan Message
	lastNum int
	size    int
}

type windowRecord struct {
	msg     Message
	isSent  bool
	isACKed bool
}

func NewWindow(chanOut chan Message, size int, lastNum int) *slidingWindow {
	w := &slidingWindow{
		window:  make([]windowRecord, 0),
		chanIn:  make(chan Message),
		chanOut: chanOut,
		chanOp:  make(chan int),
		lastNum: lastNum,
		size:    size,
	}
	go w.stateMachine()
	return w
}

func (w *slidingWindow) stateMachine() {
	for {
		op := <-w.chanOp
		switch op {
		case MESSAGE:
			msg := <-w.chanIn
			if msg.SeqNum >= w.lastNum {
				r := windowRecord{
					msg:     msg,
					isSent:  false,
					isACKed: false,
				}
				w.window = append(w.window, r)
				w.slide()
			}
		case ACK:
			msg := <-w.chanIn
			if msg.SeqNum >= w.lastNum {
				log.Printf("[S][ACK]%v ACKed, with lastnum = %v\n", w.window[msg.SeqNum-w.lastNum].msg, w.lastNum)
				w.window[msg.SeqNum-w.lastNum].isACKed = true
				w.slide()
			}
		case RESEND:
			w.resend()
		}
	}
}

func (w *slidingWindow) slide() {
	i := 0
	for ; i < len(w.window) && i < w.size; i++ {
		if !w.window[i].isACKed {
			break
		}
	}
	log.Printf("[S][Slide]Slide by %v, ans %v", i, w.window)
	w.window = w.window[i:]
	w.lastNum += i
	for i := 0; i < len(w.window) && i < w.size; i++ {
		if !w.window[i].isSent {
			w.chanOut <- w.window[i].msg
			w.window[i].isSent = true
		}
	}
}

func (w *slidingWindow) resend() {
	for i := 0; i < len(w.window) && i < w.size; i++ {
		if w.window[i].isSent && !w.window[i].isACKed {
			w.chanOut <- w.window[i].msg
		}
	}
}

type SlidingWindow_ interface {
	// Give a sending Message to sliding window
	NewMessage(msg Message)
	// Give a ACK to sliding window
	NewACK(msg Message)
}

type slidingWindow_ struct {
	window  []windowRecord
	chanOp  chan int
	chanIn  chan Message
	chanOut chan Message
	lastNum int
	size    int
}


func NewWindow_(chanOut chan Message, size int, lastNum int) *slidingWindow_ {
	w := &slidingWindow_{
		window:  make([]windowRecord, 0),
		chanIn:  make(chan Message),
		chanOut: chanOut,
		chanOp:  make(chan int),
		lastNum: lastNum,
		size:    size,
	}
	go w.stateMachine()
	return w
}

func (w *slidingWindow_) stateMachine() {
	for {
		op := <-w.chanOp
		switch op {
		case MESSAGE:
			msg := <-w.chanIn
			if msg.SeqNum >= w.lastNum {
				r := windowRecord{
					msg:     msg,
					isSent:  false,
					isACKed: false,
				}
				w.window = append(w.window, r)
				w.slide()
			}
		case ACK:
			msg := <-w.chanIn
			if msg.SeqNum >= w.lastNum {
				log.Printf("[C][ACK]%v ACKed, with lastnum = %v\n", w.window[msg.SeqNum-w.lastNum].msg, w.lastNum)
				w.window[msg.SeqNum-w.lastNum].isACKed = true
				w.slide()
			}
		case RESEND:
			w.resend()
		}
	}
}

func (w *slidingWindow_) slide() {
	i := 0
	for ; i < len(w.window) && i < w.size; i++ {
		if !w.window[i].isACKed {
			break
		}
	}
	log.Printf("[C][Slide]Slide by %v, ans %v", i, w.window)
	w.window = w.window[i:]
	w.lastNum += i
	for i := 0; i < len(w.window) && i < w.size; i++ {
		if !w.window[i].isSent {
			w.chanOut <- w.window[i].msg
			w.window[i].isSent = true
		}
	}
}

func (w *slidingWindow_) resend() {
	for i := 0; i < len(w.window) && i < w.size; i++ {
		if w.window[i].isSent && !w.window[i].isACKed {
			w.chanOut <- w.window[i].msg
		}
	}
}
