/*
 * Copyright The Kmesh Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ads

import (
	"fmt"
	"log"
	"net"
	"strings"

	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	mcp "istio.io/api/mcp/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/adsc"
	"istio.io/istio/pkg/config/schema/collections"
	"istio.io/istio/pkg/model"
	"kmesh.net/kmesh-coredns-plugin/pkg/dns"
	"kmesh.net/kmesh-coredns-plugin/pkg/options"
)

type AdsController struct {
	server    *dns.KmeshDNSServer
	adsClient *adsc.ADSC
	vip       string
}

func NewAdsController(dnsServer *dns.KmeshDNSServer) (*AdsController, error) {
	c := &AdsController{
		server: dnsServer,
		vip:    options.GetConfig().VIP,
	}
	client, err := adsc.New(options.GetConfig().XDSAddress, &adsc.ADSConfig{
		InitialDiscoveryRequests: configInitialRequests(),
		ResponseHandler:          c,
		Config: adsc.Config{
			Workload:  options.GetConfig().ServiceNode,
			Namespace: options.GetConfig().ServiceNameSpace,
			Meta: model.NodeMetadata{
				Generator: "api",
			}.ToStruct(),
			GrpcOpts: []grpc.DialOption{
				// Because we use the custom grpc options for adsc, here we should
				// explicitly set transport credentials.
				// TODO: maybe we should use the tls settings within ConfigSource
				// to secure the connection between istiod and remote xds server.
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to dial XDS %s %v", "controller.gateway-system.svc.cluster.local:15010", err)
		return nil, err
	}

	c.adsClient = client
	return c, err
}

func (c *AdsController) Start() error {
	return c.adsClient.Run()
}

func (c *AdsController) Stop() {
	c.adsClient.Close()
}

func configInitialRequests() []*discovery.DiscoveryRequest {
	out := make([]*discovery.DiscoveryRequest, 0, 1)
	out = append(out, &discovery.DiscoveryRequest{
		TypeUrl: collections.ServiceEntry.GroupVersionKind().String(),
	})

	return out
}

func (c *AdsController) HandleResponse(con *adsc.ADSC, response *discovery.DiscoveryResponse) {
	dnsEntries := make(map[string][]net.IP)
	for _, r := range response.Resources {
		m := &mcp.Resource{}
		if err := anypb.UnmarshalTo(r, m, proto.UnmarshalOptions{}); err != nil {
			fmt.Println("Error unmarshalling:", err)
			continue
		}
		e := &networking.ServiceEntry{}
		if err := anypb.UnmarshalTo(m.Body, e, proto.UnmarshalOptions{}); err != nil {
			fmt.Println("Error unmarshalling:", err)
			continue
		}
		if e.Resolution == networking.ServiceEntry_NONE {
			// NO DNS based service discovery for service entries
			// that specify NONE as the resolution. NONE implies
			// that Istio should use the IP provided by the caller
			continue
		}
		addresses := e.Addresses
		if len(addresses) == 0 && c.vip != "" {
			// If the ServiceEntry has no Addresses, map to a user-supplied default value, if provided
			addresses = []string{c.vip}
		}
		vips := convertToVIPs(addresses)
		if len(vips) == 0 {
			continue
		}
		for _, host := range e.Hosts {
			key := fmt.Sprintf("%s.", host)
			if strings.Contains(host, "*") {
				// Validation will ensure that the host is of the form *.foo.com
				parts := strings.SplitN(host, ".", 2)
				// Prefix wildcards with a . so that we can distinguish these entries in the map
				key = fmt.Sprintf(".%s.", parts[1])
			}
			dnsEntries[key] = vips
		}
	}
	c.server.UpdateDNSEntries(dnsEntries)

	// send ACK
	ackRequest := &discovery.DiscoveryRequest{
		VersionInfo:   response.VersionInfo,
		ResponseNonce: response.Nonce,
		TypeUrl:       response.TypeUrl,
	}
	if err := con.Send(ackRequest); err != nil {
		fmt.Println("Error sending ACK:", err)
	}
}

func convertToVIPs(addresses []string) []net.IP {
	vips := make([]net.IP, 0)

	for _, address := range addresses {
		// check if its CIDR.  If so, reject the address unless its /32 CIDR
		if strings.Contains(address, "/") {
			if ip, network, err := net.ParseCIDR(address); err != nil {
				ones, bits := network.Mask.Size()
				if ones == bits {
					// its a full mask (e.g., /32). Effectively an IP
					vips = append(vips, ip)
				}
			}
		} else {
			if ip := net.ParseIP(address); ip != nil {
				vips = append(vips, ip)
			}
		}
	}

	return vips
}
