package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	PREFIX = "[k8-entrypoint] "
)

type Dependency struct {
	Name     string   // IE: kafka,cassandra-aux,zookeeper
	PortName string   // The namespace to look for the endpoint in
	Hosts    []string // List of hosts given by the endpoint api for this service
	Port     string   // The port number retrieved from the endpoints api
}

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, PREFIX+"during InClusterConfig() - %s\n", err)
		os.Exit(1)
	}
	// creates the client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, PREFIX+"during NewForConfig() - %s\n", err)
		os.Exit(1)
	}

	// Determine our namespace
	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	fd, err := os.Open(namespaceFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, PREFIX+"while opening '%s' - %s\n", namespaceFile, err)
		os.Exit(1)
	}
	namespace, err := ioutil.ReadAll(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, PREFIX+"while reading '%s' - %s\n", namespaceFile, err)
		os.Exit(1)
	}

	// Collect our dependencies by reading the 'DEPENDS_ON' environment variable
	for _, dep := range getDeps() {
		// Wait for our dependencies
		if retCode := waitFor(client, string(namespace), &dep); retCode != 0 {
			os.Exit(retCode)
		}
		// Set environment vars for this dependency
		os.Setenv(fmt.Sprintf("%s_HOSTS", strings.ToUpper(dep.Name)), strings.Join(dep.Hosts, ","))
		os.Setenv(fmt.Sprintf("%s_PORT", strings.ToUpper(dep.Name)), dep.Port)
	}

	// Run the service as a child process
	retCode, err := runService()
	if err != nil {
		fmt.Fprintf(os.Stderr, PREFIX+"non-zero exit '%d' for %+v\n", retCode, os.Args[1:])
	}
	os.Exit(int(retCode))
}

func runService() (uint32, error) {
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

func getDeps() []Dependency {
	var results []Dependency

	depends := os.Getenv("DEPENDS_ON")
	if len(depends) == 0 {
		fmt.Println(PREFIX + "'DEPENDS_ON' not set or empty; skipping dep check...")
		return results
	}

	for _, item := range strings.Split(depends, ",") {
		parts := strings.Split(item, ":")

		// If the dependency includes a port name
		if len(parts) > 1 {
			results = append(results, Dependency{Name: parts[0], PortName: parts[1]})
		} else {
			results = append(results, Dependency{Name: parts[0]})
		}
	}
	return results
}

// Wait for the ip or hostname to show up in the endpoints api.
func waitFor(client *kubernetes.Clientset, namespace string, dep *Dependency) int {
	for {
		fmt.Printf(PREFIX+"Looking for endpoint '%s' in namespace '%s' and port name '%s'", dep.Name, namespace, dep.PortName)
		endpoint, err := client.CoreV1().Endpoints(namespace).Get(dep.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Printf("-- Not found\n")
			time.Sleep(time.Second * 3)
			continue
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Printf(" -- k8 endpoint error %v\n", statusError.ErrStatus.Message)
			return 1
		} else if err != nil {
			fmt.Printf("-- k8 http error %v\n", err)
			return 1
		}

		if len(endpoint.Subsets) == 0 {
			fmt.Print(" -- Not Found\n")
			time.Sleep(time.Second * 3)
			continue
		}
		// Find the port requested
		for i, subset := range endpoint.Subsets {
			for _, port := range subset.Ports {
				if port.Name == dep.PortName {
					dep.Hosts = hostsFromSubset(endpoint.Subsets[i])
					dep.Port = fmt.Sprintf("%d", port.Port)
					fmt.Print(" -- Found\n")
					return 0
				}
			}
		}
		fmt.Print(" -- Port Not Found\n")
		time.Sleep(time.Second * 3)
	}
	return 0
}

func hostsFromSubset(subset v1.EndpointSubset) (results []string) {
	for _, address := range subset.Addresses {
		if address.Hostname != "" {
			results = append(results, address.Hostname)
		} else {
			results = append(results, address.IP)
		}
	}
	return results
}
