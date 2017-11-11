package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/cmu440/lsp"
	"encoding/json"
	"github.com/cmu440/bitcoin"
)

func main() {
	const numArgs = 4
	if len(os.Args) != numArgs {
		fmt.Printf("Usage: ./%s <hostport> <message> <maxNonce>", os.Args[0])
		return
	}
	hostport := os.Args[1]
	message := os.Args[2]
	maxNonce, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil {
		fmt.Printf("%s is not a number.\n", os.Args[3])
		return
	}

	client, err := lsp.NewClient(hostport, lsp.NewParams())
	if err != nil {
		fmt.Println("Failed to connect to server:", err)
		return
	}

	defer client.Close()

	buf, err := json.Marshal(bitcoin.NewRequest(message, 0, maxNonce))
	if err != nil {
		fmt.Println("Message Error!", message)
		return
	}

	err = client.Write(buf)
	if err != nil {
		printDisconnected()
		return
	}

	buf, err = client.Read()
	if err != nil {
		printDisconnected()
		return
	}
	result := new(bitcoin.Message)
	json.Unmarshal(buf, result)

	printResult(result.Hash, result.Nonce)
}

// printResult prints the final result to stdout.
func printResult(hash, nonce uint64) {
	fmt.Println("Result", hash, nonce)
}

// printDisconnected prints a disconnected message to stdout.
func printDisconnected() {
	fmt.Println("Disconnected")
}
