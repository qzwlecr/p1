package main

import (
	"fmt"
	"os"

	"github.com/cmu440/lsp"
	"github.com/cmu440/bitcoin"
	"encoding/json"
	"math"
)

// Attempt to connect miner as a client to the server.
func joinWithServer(hostport string) (lsp.Client, error) {
	cli, err := lsp.NewClient(hostport, lsp.NewParams())
	if err != nil {
		return nil, err
	}

	msg := bitcoin.NewJoin()
	buf, _ := json.Marshal(msg)

	err = cli.Write(buf)

	if err != nil {
		cli.Close()
		return nil, err
	}

	return cli, nil
}

func main() {
	const numArgs = 2
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport>", os.Args[0])
		return
	}

	hostport := os.Args[1]
	miner, err := joinWithServer(hostport)
	if err != nil {
		fmt.Println("Failed to join with server:", err)
		return
	}

	defer miner.Close()

	for {
		buf, err := miner.Read()
		if err != nil {
			return
		}
		req := new(bitcoin.Message)
		json.Unmarshal(buf, req)
		var min, minIndex uint64 = math.MaxUint64, 0
		for i := req.Lower; i <= req.Upper; i++ {
			res := bitcoin.Hash(req.Data, i)
			if res < min {
				min = res
				minIndex = i
			}
		}

		res := bitcoin.NewResult(min, minIndex)
		buf, _ = json.Marshal(res)
		err = miner.Write(buf)
		if err != nil {
			return
		}
	}

}
