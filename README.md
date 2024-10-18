# kmesh-coredns-plugin

The plugin runs as a standlone server in the cluster, serving DNS A records over gRPC to CoreDNS.

# Quick start

## Step 1: Install kubectl & kind

Linux:
```sh
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.18.0/kind-linux-amd64
chmod +x ./kubectl ./kind
sudo mv ./kubectl ./kind /usr/local/bin
```

## Step 2: Create and Activate kind
```sh
kind create cluster --name kmesh-dns
```

## Step3: Install Istio
please refer to [Install istio](https://istio.io/latest/docs/setup/getting-started/#install)

## Step4: Deploy Kmesh coredns plugin
```sh
kubectl apply -f manifest/deploy.yaml
```

## Step5: Forward coreDNS to Kmesh coredns plugin
```sh
kubectl edit cm coredns -n kube-system
```

```yaml
apiVersion: v1
data:
  Corefile: |
    .:53 {
        errors
        health {
           lameduck 5s
        }
        ready
        kubernetes cluster.local in-addr.arpa ip6.arpa {
           pods insecure
           fallthrough in-addr.arpa ip6.arpa
           ttl 30
        }
        prometheus :9153
        forward . /etc/resolv.conf {
           max_concurrent 1000
        }
        cache 30
        loop
        reload
        loadbalance
    }
    // put the domain surfix here
    // e.g. for consul service 
    consul.local:53 {
        errors
        cache 30
        // the KMESHPLUGIN_IP is the kmesh coredns plugin service clusterip
        forward . $KMESHPLUGIN_IP:15053
    }
    // e.g. for nacos service 
    nacos:53 {
        errors
        cache 30
        // the KMESHPLUGIN_IP is the kmesh coredns plugin service clusterip
        forward . $KMESHPLUGIN_IP:15053
    }
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
```