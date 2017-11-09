package lsp

import "log"

type Sorter interface {
	Add(message Message)
}

type sorter struct {
	chanIn     chan Message
	chanOut    chan Message
	msgMap     map[int]Message
	nextSeqNum int
}

func NewSorter(chanOut chan Message, nextSeqNum int) *sorter {
	st := &sorter{
		chanIn:     make(chan Message),
		chanOut:    chanOut,
		msgMap:     make(map[int]Message),
		nextSeqNum: nextSeqNum,
	}
	go st.maintain()
	return st
}

func (st *sorter) maintain() {
	for {
		select {
		case msg := <-st.chanIn:
			if msg.SeqNum < st.nextSeqNum {
				continue
			}
			if _, ok := st.msgMap[msg.SeqNum]; ok {
				continue
			}
			if msg.SeqNum == st.nextSeqNum {
				st.chanOut <- msg
				st.nextSeqNum ++
				for {
					if msg, ok := st.msgMap[st.nextSeqNum]; ok {
						delete(st.msgMap, st.nextSeqNum)
						st.nextSeqNum ++
						st.chanOut <- msg
					} else {
						break
					}
				}
				continue
			}
			st.msgMap[msg.SeqNum] = msg
		}
	}
}

func (st *sorter) Add(msg Message) {
	st.chanIn <- msg
	return
}

const (
	MESSAGE = iota
	ACK
	RESEND
)

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
