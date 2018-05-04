package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	go func() {
		var sigs = make(chan os.Signal, 3)
		signal.Notify(sigs)
		for {
			select {
			case sig := <-sigs:
				switch sig {
				case syscall.SIGUSR2:
					fmt.Printf("SIGUSR2\n")
				case syscall.SIGUSR1:
					fmt.Printf("SIGUSR1\n")
				case syscall.SIGTERM:
					fmt.Printf("SIGTERM\n")
					os.Exit(0)
				}
			}
		}
	}()

	for _, env := range os.Environ() {
		fmt.Println(env)
	}
	fmt.Printf("Sleeping....\n")
	time.Sleep(time.Hour)
}
