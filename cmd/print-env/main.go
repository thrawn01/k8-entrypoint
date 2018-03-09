package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	for _, env := range os.Environ() {
		fmt.Println(env)
	}
	fmt.Printf("Sleeping....\n")
	time.Sleep(time.Hour)
}
