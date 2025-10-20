# kmesh-coredns-plugin
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkmesh-net%2Fkmesh-dns-plugin.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkmesh-net%2Fkmesh-dns-plugin?ref=badge_shield)


The plugin runs as a standlone server in the cluster, serving DNS A records over gRPC to CoreDNS.

# Quick start

## Step1: Deploy Kmesh coredns plugin
```sh
kubectl apply -f manifest/deploy.yaml
```

## Step2: Forward coreDNS to Kmesh coredns plugin
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
    // put the domain suffix here
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

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fkmesh-net%2Fkmesh-dns-plugin.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fkmesh-net%2Fkmesh-dns-plugin?ref=badge_large)