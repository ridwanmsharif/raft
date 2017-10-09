package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ridwanmsharif/raft/raft"
)

func main() {
	a := []int64{}
	storage := raft.NewStorage()
	c := raft.Config{1, a, 2, 1, storage, 1, log.New(os.Stdout, "Log: ", 2)}
	fmt.Println(c.Validate())
	fmt.Println("vim-go")
}
