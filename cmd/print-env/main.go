package main

import (
	"fmt"
	"io/ioutil"
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
	service := os.Getenv("SERVICE_NAME")
	if len(service) != 0 {
		configFile := fmt.Sprintf("/etc/mailgun/%s/config.yaml", service)
		fd, err := os.Open(configFile)
		if err != nil {
			fmt.Printf("while opening config file '%s' - '%s'", configFile, err)
			goto sleep
		}
		contents, err := ioutil.ReadAll(fd)
		if err != nil {
			fmt.Printf("while reading config file '%s' - '%s'", configFile, err)
			goto sleep
		}
		fmt.Printf("-------- %s -----------\n", configFile)
		fmt.Printf("%s", string(contents))
		fmt.Printf("-----------------------\n")
	}

sleep:
	fmt.Printf("Sleeping....\n")
	time.Sleep(time.Hour)
}
