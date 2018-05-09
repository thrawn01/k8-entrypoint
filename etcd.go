package entrypoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"gopkg.in/yaml.v2"
)

func GetConfig() error {
	var cli *clientv3.Client
	var err error

	service := os.Getenv("SERVICE_NAME")
	if len(service) == 0 {
		fmt.Printf(PREFIX + "envronment variable 'SERVICE_NAME' not defined; skipping fetch config from etcd\n")
		return nil
	}

	datacenter := os.Getenv("DC_SHORT_NAME")
	if len(service) == 0 {
		fmt.Printf(PREFIX + "environment variable 'DC_SHORT_NAME' not defined; skipping fetch config from etcd\n")
		return nil
	}

	for {
		cli, err = clientv3.New(clientv3.Config{
			Endpoints:   getEtcdEndpoints(),
			DialTimeout: 2 * time.Second,
		})

		if err != nil {
			fmt.Printf(PREFIX+"while connecting to etcd v3 - '%s'; retrying...", err)
			time.Sleep(time.Second * 3)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		value, err := cli.Get(ctx, fmt.Sprintf("/mailgun/configs/%s/%s", datacenter, service), nil)
		cancel()

		if err != nil {
			fmt.Printf(PREFIX+"while fetching keys from etcd v3 - '%s'; retrying...", err)
			time.Sleep(time.Second * 3)
			cli.Close()
			continue
		}

		// Read in JSON config
		config := make(map[interface{}]interface{})
		err = json.Unmarshal(value.Kvs[0].Value, &config)
		if err != nil {
			fmt.Printf(PREFIX+"while marshalling config - '%s'; retrying...", err)
			time.Sleep(time.Second * 3)
			cli.Close()
			continue
		}

		// Write out the YAML config to disk
		out, err := yaml.Marshal(&config)
		if err != nil {
			fmt.Printf(PREFIX+"while marshalling config yaml - '%s'; retrying...", err)
			time.Sleep(time.Second * 3)
			cli.Close()
			continue
		}

		configFile := fmt.Sprintf("/etc/mailgun/%s/config.yaml", service)
		fd, err := os.Open(configFile)
		if err != nil {
			fmt.Printf(PREFIX+"while openning config file '%s' - '%s'; retrying...", configFile, err)
			time.Sleep(time.Second * 3)
			cli.Close()
			continue
		}

		_, err = fd.Write(out)
		if err != nil {
			fmt.Printf(PREFIX+"while writing config to file '%s' - '%s'; retrying...", configFile, err)
			time.Sleep(time.Second * 3)
			cli.Close()
			continue
		}
		fd.Close()
	}
	return nil
}

func getEtcdEndpoints() []string {
	// Environment variables should be a command separated list or singular of `host:port`
	endpoints := os.Getenv("ETCD_V3_ENDPOINTS")
	if len(endpoints) == 0 {
		// Use the kubernetes service name if none provided
		return []string{"etcd-cluster-client:2379"}
	}

	var results []string
	for _, item := range strings.Split(endpoints, ",") {
		results = append(results, item)
	}
	return results
}
