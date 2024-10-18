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

package dns

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"

	logger "istio.io/istio/pkg/log"
)

var log = logger.RegisterScope("kmesh-dns", "kmesh-dns dns")

type KmeshDNSServer struct {
	dnsProxies []*dnsProxy
	mapMutex   sync.RWMutex
	dnsEntries map[string][]net.IP
}

func NewDNSServer(addr string) (*KmeshDNSServer, error) {
	h := &KmeshDNSServer{}
	if addr == "" {
		addr = "localhost:15053"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("dns address must be a valid host:port: %v", err)
	}
	addresses := []string{addr}
	if host == "localhost" {
		addresses = []string{}
		addresses = append(addresses, net.JoinHostPort("0.0.0.0", port))
	}

	for _, ipAddr := range addresses {
		for _, proto := range []string{"udp", "tcp"} {
			proxy, err := newDNSProxy(proto, ipAddr, h)
			if err != nil {
				return nil, err
			}
			h.dnsProxies = append(h.dnsProxies, proxy)
		}
	}
	return h, nil
}

func (h *KmeshDNSServer) Start() {
	for _, p := range h.dnsProxies {
		go p.start()
	}
}

func (h *KmeshDNSServer) Stop() {
	for _, p := range h.dnsProxies {
		p.close()
	}
}

func (h *KmeshDNSServer) ServeDNS(proxy *dnsProxy, w dns.ResponseWriter, req *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(req)
	response.Authoritative = true
	if len(req.Question) == 0 {
		response = new(dns.Msg)
		response.Rcode = dns.RcodeServerFailure
		_ = w.WriteMsg(response)
		return
	}

	for _, q := range req.Question {
		switch q.Qtype {
		case dns.TypeA:
			var vips []net.IP
			log.Debugf("Query A record: %s->%v\n", q.Name, q)
			h.mapMutex.RLock()
			//log.Printf("DNS map: %v\n", h.dnsEntries)
			if h.dnsEntries != nil {
				vips = h.dnsEntries[q.Name]
				if vips == nil {
					// check for wildcard format
					// Split name into pieces by . (remember that DNS queries have dot in the end as well)
					// Check for each smaller variant of the name, until we have
					pieces := strings.Split(q.Name, ".")
					pieces = pieces[1:]
					for ; len(pieces) > 2; pieces = pieces[1:] {
						if vips = h.dnsEntries[fmt.Sprintf(".%s", strings.Join(pieces, "."))]; vips != nil {
							break
						}
					}
				}
			}
			h.mapMutex.RUnlock()
			if vips != nil {
				log.Debugf("Found %s->%v\n", q.Name, vips)
				response.Answer = a(q.Name, vips)
			}
		}
	}
	if len(response.Answer) == 0 {
		log.Debugf("Could not find the service requested")
		response.Rcode = dns.RcodeNameError
	}

	_ = w.WriteMsg(response)
}

func a(zone string, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.A)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: 3600}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}

func (h *KmeshDNSServer) UpdateDNSEntries(dnsEntries map[string][]net.IP) {
	h.mapMutex.Lock()
	h.dnsEntries = make(map[string][]net.IP)
	for k, v := range dnsEntries {
		log.Debugf("adding DNS mapping: %s->%v\n", k, v)
		h.dnsEntries[k] = v
	}
	h.mapMutex.Unlock()
}
