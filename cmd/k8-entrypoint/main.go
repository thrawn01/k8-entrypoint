package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
		fmt.Fprintf(os.Stderr, "during InClusterConfig() - %s", err)
		os.Exit(1)
	}
	// creates the client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "during NewForConfig() - %s\n", err)
		os.Exit(1)
	}

	// Determine our namespace
	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	fd, err := os.Open(namespaceFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "while opening '%s' - %s\n", namespaceFile, err)
		os.Exit(1)
	}
	namespace, err := ioutil.ReadAll(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "while reading '%s' - %s\n", namespaceFile, err)
		os.Exit(1)
	}

	// Collect our dependencies by reading the 'DEPENDS_ON' environment variable
	for _, dep := range getDeps() {
		// Wait for our dependencies
		if err := waitFor(client, string(namespace), &dep); err != nil {
			panic(err.Error())
		}
		// Set environment vars for this dependency
		os.Setenv(fmt.Sprintf("%s_HOSTS", strings.ToUpper(dep.Name)), strings.Join(dep.Hosts, ","))
		os.Setenv(fmt.Sprintf("%s_PORT", dep.Name), dep.Port)
	}

	// Execute our entry-point
	err = syscall.Exec(os.Args[1], os.Args[1:], os.Environ())
}

func getDeps() []Dependency {
	var results []Dependency

	depends := os.Getenv("DEPENDS_ON")
	if len(depends) == 0 {
		fmt.Fprintln(os.Stderr, "'DEPENDS_ON' not set or empty")
		os.Exit(1)
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
func waitFor(client *kubernetes.Clientset, namespace string, dep *Dependency) error {
	for {
		fmt.Printf("Waiting for endpoint '%s' in '%s' and port name '%s'", dep.Name, namespace, dep.PortName)
		endpoint, err := client.CoreV1().Endpoints(namespace).Get(dep.Name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "endpoint for '%s' in namespace '%s' not found\n", dep.Name, namespace)
			time.Sleep(time.Second)
			continue
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			fmt.Fprintf(os.Stderr, "k8 endpoint error %v\n", statusError.ErrStatus.Message)
			os.Exit(1)
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "k8 http error %v\n", err)
			os.Exit(1)
		}

		if len(endpoint.Subsets) == 0 {
			fmt.Print(" -- Not Found\n")
			time.Sleep(time.Second)
		}
		// Find the port requested
		for i, subset := range endpoint.Subsets {
			for _, port := range subset.Ports {
				if port.Name == dep.PortName {
					dep.Hosts = hostsFromSubset(endpoint.Subsets[i])
					dep.Port = fmt.Sprintf("%d", port.Port)
					fmt.Print(" -- Found\n")
					return nil
				}
			}
		}
		fmt.Print(" -- Port Not Found\n")
		time.Sleep(time.Second)
	}
	return nil
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
