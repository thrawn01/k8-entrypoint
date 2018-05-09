package entrypoint

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func RunService() (uint32, error) {
	// Start zombie reaper
	go zombieReaper()

	// Create the process that starts the service
	c := exec.Command(os.Args[1], os.Args[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin

	// Allow kill signals to reach children of the service
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Start signal forwarder
	go sigForwarder(c)

	// Run and extract the return code
	err := c.Run()
	if err != nil {
		// If the return code is != 0
		if exit, ok := err.(*exec.ExitError); ok {
			if retCode, ok := exit.Sys().(syscall.WaitStatus); ok {
				return uint32(retCode), err
			}
		}
		return 255, err
	}
	return 0, err
}

func sigForwarder(c *exec.Cmd) {
	var sigs = make(chan os.Signal, 3)
	signal.Notify(sigs)
	for {
		select {
		case sig := <-sigs:
			switch sig {
			// SIGTERM is the default signal sent to kill pid 1 in a container, as defined
			// by OCI and Docker. You can override the default signal by using `STOPSIGNAL`
			// in your Dockerfile
			case syscall.SIGTERM:
				// First forward syscall.SIGUSR1 to tell scroll/rifle to un-register vulcand routes
				fmt.Printf(PREFIX + "Caught SIGTERM; sending USR1 signal to children, Waiting 3 seconds...\n")
				syscall.Kill(-c.Process.Pid, syscall.SIGUSR1)
				// Then forward syscall.SIGTERM to all children
				go func() {
					time.Sleep(time.Second * 3)
					fmt.Printf(PREFIX + "Sending SIGTERM signal to children\n")
					syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
				}()
			default:
				// Forward all other signals to all children
				syscall.Kill(-c.Process.Pid, sig.(syscall.Signal))
			}
		}
	}

	// Kill group
	syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}

func zombieReaper() {
	var sigs = make(chan os.Signal, 3)
	signal.Notify(sigs, syscall.SIGCHLD)

	for {
		select {
		case sig := <-sigs:
			fmt.Printf(PREFIX+"Received SIGCHLD -'%+v'\n", sig)
			for {
				var status syscall.WaitStatus

				// -1 pid means "wait for any child process"
				// syscall.WNOHANG means "return immediately if no child has exited"
				pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)

				// System call was interrupted, try again until we succeed.
				for err == syscall.EINTR {
					pid, err = syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
				}

				// No child processes found to reap
				if err == syscall.ECHILD {
					break
				}
				fmt.Printf(PREFIX+"Reaped Zombie process pid=%d, exit status=%+v\n", pid, status)
			}
		}
	}
}
