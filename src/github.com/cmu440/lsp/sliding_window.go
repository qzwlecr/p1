package lsp

import "fmt"

type SlidingWindow interface {
	// Give a sending Message to sliding window
	NewMessage(msg Message)
	// Give a ACK to sliding window
	NewACK(msg Message)
}

type slidingWindow struct {
	window  []windowRecord
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
		chanOut: chanOut,
		lastNum: lastNum,
		size:    size,
	}
	return w
}

func (w *slidingWindow) NewMessage(msg Message) {
	r := windowRecord{
		msg:     msg,
		isSent:  false,
		isACKed: false,
	}
	w.window = append(w.window, r)
	w.slide()
}

func (w *slidingWindow) NewACK(msg Message) {
	if msg.SeqNum < w.lastNum {
		return
	}
	fmt.Printf("[ACK]%v ACKed, with lastnum = %v\n", msg.SeqNum, w.lastNum)
	w.window[msg.SeqNum-w.lastNum].isACKed = true
}

func (w *slidingWindow) slide() {
	i := 0
	for ;i < len(w.window) && i < w.size;i++ {
		if !w.window[i].isACKed {
			break
		}
	}
	w.window = w.window[i:]
	w.lastNum += i
	w.send()
}

func (w *slidingWindow) send() {
	for i := 0; i < len(w.window) && i < w.size; i++ {
		if !w.window[i].isSent {
			w.chanOut <- w.window[i].msg
			fmt.Printf("[Slide]%v Sent, with lastnum = %v\n", w.window[i].msg, w.lastNum)
			w.window[i].isSent = true
		}
	}
}
