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
and the name of port it is interested in.  Additionally users can specify a
ready check with the flag `--ready` which when called with return 0 if the
dependencies were resolved and the service started or 1 if not.

### ETCD configs
Many of our services have configuration stored in etcd. If the environment
vairable `SERVICE_NAME` and `DC_SHORT_NAME` is defined, `k8-entrypoint` will
connect to etcd via the `ETCD_V3_ENDPOINTS` variable or if not provided will
default to using the dns name `etcd-cluster-client:2379` to retrieve the etcd
config and write the config in yaml format inside the local container at
`/etc/mailgun/<service_name>/config.yaml` . This works exactly like
`mailgun/deploy` currently and should work with all golang based mailgun
containers.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-service
spec:
      containers:
      - name: my-service
        image: my-service:latest
        readinessProbe:
          exec:
            command:
            - /usr/sbin/k8-entrypoint
            - --ready
          initialDelaySeconds: 1
          periodSeconds: 5
        env:
          - name: DEPENDS_ON
            value: "etcd-cluster-client:client,kafka:client,zookeeper:server"
          - name: SERVICE_NAME
            value: "scout"
          - name: DC_SHORT_NAME
            value: "devgun"

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
