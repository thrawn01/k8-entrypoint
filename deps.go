package entrypoint

import (
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func GetDeps() []Dependency {
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
func WaitFor(client *kubernetes.Clientset, namespace string, dep *Dependency) int {
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
