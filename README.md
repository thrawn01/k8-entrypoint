## What is?
This package provides an entrypoint that will wait for k8 endpoints to be
available. The advantage in waiting for endpoints is that services are not
available as endpoints unless the pod hosting the service has returned a
positive `readinessProbe` or `livenessProbe` which is a good indicator the
service is ready to serve requests.

## How do?
A service spec places the entrypoint first `/k8-entrypoint /path/to/service
arg1 arg2` then declares comma separated dependencies by adding an environment
variable called `DEPENDS_ON`. Each dependency declares the name of the service
and the name of port it is interested in.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-service
spec:
      containers:
      - name: my-service
        image: my-service:latest
        env:
          - name: DEPENDS_ON
            value: "kafka:client,zookeeper:server"
        args:
        - /bin/k8-entrypoint
        - /bin/my-service
        - -config
        - /etc/my-service.conf
```
In addition to waiting for the dependent services to be ready, `k8-entrypoint`
will place the names of the hosts it finds into environment variables that are
passed to the service.

For the example yaml above, once kafka and zookeeper are available the
following environment variables will be set.
```
KAFKA_HOSTS=kafka-0,kafka-1,kafka-2
KAFKA_PORT=9092
ZOOKEEPER_HOSTS=zookeeper-0,zookeeper-1,zookeeper-2
ZOOKEEPER_PORT=2888
```

This is advantageous as K8 does not include environment variables in pods for
services without a `ClusterIP` which many of our services currently require.

This is only a temporary solution, eventually our services should query DNS for
services and not expect a list of nodes to be passed in as config options.
