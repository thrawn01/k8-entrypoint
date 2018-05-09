package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/mailgun/k8-entrypoint"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, entrypoint.PREFIX+"during InClusterConfig() - %s\n", err)
		os.Exit(1)
	}
	// creates the client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, entrypoint.PREFIX+"during NewForConfig() - %s\n", err)
		os.Exit(1)
	}

	// Determine our namespace
	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	fd, err := os.Open(namespaceFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, entrypoint.PREFIX+"while opening '%s' - %s\n", namespaceFile, err)
		os.Exit(1)
	}
	namespace, err := ioutil.ReadAll(fd)
	if err != nil {
		fmt.Fprintf(os.Stderr, entrypoint.PREFIX+"while reading '%s' - %s\n", namespaceFile, err)
		os.Exit(1)
	}

	// Collect our dependencies by reading the 'DEPENDS_ON' environment variable
	for _, dep := range entrypoint.GetDeps() {
		// Wait for our dependencies
		if retCode := entrypoint.WaitFor(client, string(namespace), &dep); retCode != 0 {
			os.Exit(retCode)
		}
		// Set environment vars for this dependency
		os.Setenv(fmt.Sprintf("%s_HOSTS", strings.ToUpper(dep.Name)), strings.Join(dep.Hosts, ","))
		os.Setenv(fmt.Sprintf("%s_PORT", strings.ToUpper(dep.Name)), dep.Port)
	}

	// TODO: fetch config if needed
	entrypoint.GetConfig()

	// Run the service as a child process
	retCode, err := entrypoint.RunService()
	if err != nil {
		fmt.Fprintf(os.Stderr, entrypoint.PREFIX+"non-zero exit '%d' for %+v\n", retCode, os.Args[1:])
	}
	os.Exit(int(retCode))
}
