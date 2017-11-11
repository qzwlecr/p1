package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/cmu440/lsp"
	"github.com/cmu440/bitcoin"
	"container/list"
	"encoding/json"
)

type server struct {
	lspServer    lsp.Server
	miners       map[int]bool
	minersClient map[int]int
	clients      map[int]int
	jobs         map[int]*bitcoin.Message
	jobPool      *list.List
	connPool     *list.List
}

func startServer(port int) (*server, error) {
	srv := new(server)

	s, err := lsp.NewServer(port, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	srv.lspServer = s
	srv.clients = make(map[int]int)
	srv.miners = make(map[int]bool)
	srv.minersClient = make(map[int]int)
	srv.jobs = make(map[int]*bitcoin.Message)
	srv.jobPool = list.New()
	srv.connPool = list.New()
	return srv, nil
}

var LOGF *log.Logger

func main() {
	// You may need a logger for debug purpose
	const (
		name = "log.txt"
		flag = os.O_RDWR | os.O_CREATE
		perm = os.FileMode(0666)
	)

	file, err := os.OpenFile(name, flag, perm)
	if err != nil {
		return
	}
	defer file.Close()

	LOGF = log.New(file, "", log.Lshortfile|log.Lmicroseconds)
	// Usage: LOGF.Println() or LOGF.Printf()

	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <port>", os.Args[0])
		return
	}

	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Port must be a number:", err)
		return
	}

	srv, err := startServer(port)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("Server listening on port", port)

	defer srv.lspServer.Close()

	for {
		msg := new(bitcoin.Message)
		connId, buf, err := srv.lspServer.Read()
		if err != nil {
			for i, _ := range srv.miners {
				if connId == i {
					delete(srv.miners, i)
					cli := srv.minersClient[i]
					delete(srv.minersClient, i)
					delete(srv.clients, cli)
					msg := srv.jobs[cli]

					flag := false
					for j, ok := range srv.miners {
						if ok {
							buf, _ := json.Marshal(*msg)
							srv.lspServer.Write(connId, buf)
							srv.jobs[cli] = msg
							srv.clients[cli] = j
							srv.miners[j] = false
							srv.minersClient[j] = cli
							flag = true
							break
						}
					}
					if flag {
						srv.jobPool.PushBack(msg)
						srv.connPool.PushBack(connId)
					}
					break
				}
			}
		}

		json.Unmarshal(buf, msg)
		switch msg.Type {
		case bitcoin.Request:
			flag := false
			for i, ok := range srv.miners {
				if ok {
					buf, _ := json.Marshal(msg)
					err := srv.lspServer.Write(i, buf)
					if err != nil {
						delete(srv.miners, i)
						srv.lspServer.CloseConn(i)
						continue
					}

					srv.miners[i] = false
					srv.minersClient[i] = connId
					flag = true
					break
				}
			}
			if !flag {
				srv.jobPool.PushBack(msg)
				srv.connPool.PushBack(connId)
			}
		case bitcoin.Result:
			buf, _ := json.Marshal(msg)
			err := srv.lspServer.Write(srv.minersClient[connId], buf)
			if err != nil {
				srv.lspServer.CloseConn(srv.minersClient[connId])
				delete(srv.clients, srv.minersClient[connId])
			}

			delete(srv.jobs, connId)
			delete(srv.minersClient, connId)
			delete(srv.clients, connId)
			srv.miners[connId] = true
		default:
			srv.miners[connId] = true
			if srv.jobPool.Len() != 0 {
				reqId := srv.connPool.Front().Value.(int)
				msg := srv.jobPool.Front().Value.(*bitcoin.Message)
				buf, _ := json.Marshal(msg)
				srv.lspServer.Write(connId, buf)
				srv.minersClient[connId] = reqId
				srv.clients[reqId] = connId
				srv.jobs[reqId] = msg
				srv.jobPool.Remove(srv.jobPool.Front())
				srv.connPool.Remove(srv.connPool.Front())
				srv.miners[connId] = false
			}
		}
	}

}
