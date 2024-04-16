# Troubleshooting

This guide explains general info on how to debug issues if a cluster creation fails.

## providerClient authentication err

If you are using https, and when you encounter issues like:

```
kubectl logs -n cspoo-system logs -l control-plane=capo-controller-manager <cspo-controller-manager-pod>
...
[manager] 2024-04-15T15:20:07Z	DEBUG	events	Post "https://10.0.3.15/identity/v3/auth/tokens": tls: failed to verify certificate: x509: certificate signed by unknown authority	{"type": "Warning", "object": {"kind":"OpenStackNodeImageRelease","namespace":"cluster","name":"openstack-ferrol-1-27-ubuntu-capi-image-v1.27.8-v2","uid":"93d2c1c8-5a19-45f8-9f93-8e8bd5227ebf","apiVersion":"infrastructure.clusterstack.x-k8s.io/v1alpha1","resourceVersion":"3773"}, "reason": "OpenStackProviderClientNotSet"}
...
```

you must specify the CA certificate in your secret, which contains the access data to the OpenStack instance, then secret should look similar to this example:

 ```bash
apiVersion: v1
data:
  caCert: <PEM_ENCODED_CA_CERT>
  clouds.yaml: <ENCODED_CLOUDS_YAML>
kind: Secret
metadata:
  labels:
    clusterctl.cluster.x-k8s.io/move: "true"
  name: "openstack"
  namespace: cluster
 ```
