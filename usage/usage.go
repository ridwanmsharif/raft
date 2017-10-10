package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ridwanmsharif/raft/raft"
)

func main() {
	a := []int64{}
	storage := raft.NewStorage()
	c := raft.Config{1, a, 2, 1, storage, 1, log.New(os.Stdout, "Log: ", 2)}
	fmt.Println(c.Validate())
	p := []raft.Peer{}
	n := raft.StartNode(&c, p)
	Ticker := time.NewTicker(1 * time.Second)
	done := time.NewTicker(50 * time.Second)
	for {
		select {
		case <-Ticker.C:
			n.Tick()
		case _ = <-n.Ready():
			log.Printf("ready")
		case <-done.C:
			n.Stop()
			return
		}
	}
}
