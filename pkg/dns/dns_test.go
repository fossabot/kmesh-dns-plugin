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
	 "testing"
	 "time"
 
	 "github.com/miekg/dns"
	 "go.uber.org/atomic"
	 "istio.io/istio/pkg/test"
 )
 
 func TestDNS(t *testing.T) {
	 d := initDNS(t)
	 testDNS(t, d)
 }
 
 func BenchmarkDNS(t *testing.B) {
	 s := initDNS(t)
	 t.Run("nacos-format", func(b *testing.B) {
		 bench(b, s.dnsProxies[0].Address(), "test.default.public.nacos.")
	 })
 }
 
 func testDNS(t *testing.T, d *KmeshDNSServer) {
	 testCases := []struct {
		 name                    string
		 host                    string
		 id                      int
		 expectResolutionFailure int
		 expected                []dns.RR
	 }{
		 {
			 name:     "success: nacos format host",
			 host:     "test.default.public.nacos.",
			 expected: a("test.default.public.nacos.", []net.IP{net.ParseIP("1.1.1.1")}),
		 },
		 {
			 name:                    "failure: ",
			 host:                    "details.ns2.",
			 expectResolutionFailure: dns.RcodeNameError, // on home machines, the ISP may resolve to some generic webpage. So this test may fail on laptops
		 },
	 }
 
	 clients := []dns.Client{
		 {
			 Timeout: 3 * time.Second,
			 Net:     "udp",
			 UDPSize: 65535,
		 },
		 {
			 Timeout: 3 * time.Second,
			 Net:     "tcp",
		 },
	 }
	 addresses := []string{
		 d.dnsProxies[0].Address(),
		 d.dnsProxies[1].Address(),
	 }
	 currentID := atomic.NewInt32(0)
	 oldID := dns.Id
	 dns.Id = func() uint16 {
		 return uint16(currentID.Inc())
	 }
	 defer func() { dns.Id = oldID }()
	 for i := range clients {
		 addr := addresses[i]
		 for _, tt := range testCases {
			 // Test is for explicit network
			 if (strings.HasPrefix(tt.name, "udp") || strings.HasPrefix(tt.name, "tcp")) && !strings.HasPrefix(tt.name, clients[i].Net) {
				 continue
			 }
			 t.Run(clients[i].Net+"-"+tt.name, func(t *testing.T) {
				 m := new(dns.Msg)
				 q := dns.TypeA
				 m.SetQuestion(tt.host, q)
				 if tt.id != 0 {
					 currentID.Store(int32(tt.id))
					 defer func() { currentID.Store(0) }()
				 }
				 res, _, err := clients[i].Exchange(m, addr)
 
				 if err != nil {
					 t.Errorf("Failed to resolve query for %s: %v", tt.host, err)
				 } else {
					 if tt.expectResolutionFailure > 0 && tt.expectResolutionFailure != res.Rcode {
						 t.Errorf("expected resolution failure does not match with response code for %s: expected: %v, got: %v",
							 tt.host, tt.expectResolutionFailure, res.Rcode)
					 }
					 for _, answer := range res.Answer {
						 if answer.Header().Class != dns.ClassINET {
							 t.Errorf("expected class INET for all responses, got %+v for host %s", answer.Header(), tt.host)
						 }
 
						 if !equalsDNSrecords(res.Answer, tt.expected) {
							 t.Errorf("dns responses for %s do not match. \n got %v\nwant %v", tt.host, res.Answer, tt.expected)
						 }
					 }
				 }
			 })
		 }
	 }
 }
 
 func initDNS(t test.Failer) *KmeshDNSServer {
	 testAgentDNS, err := NewDNSServer("")
	 if err != nil {
		 t.Fatal(err)
	 }
	 stop := make(chan struct{})
	 testAgentDNS.Start(stop)
 
	 dnsEntries := make(map[string][]net.IP)
 
	 for host, addresses := range map[string][]string{
		 "test.default.public.nacos": {"1.1.1.1"},
	 } {
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
		 key := fmt.Sprintf("%s.", host)
		 dnsEntries[key] = vips
	 }
 
	 testAgentDNS.UpdateDNSEntries(dnsEntries)
 
	 return testAgentDNS
 }
 
 // reflect.DeepEqual doesn't seem to work well for dns.RR
 // as the Rdlength field is not updated in the a(), or aaaa() calls.
 // so zero them out before doing reflect.Deepequal
 func equalsDNSrecords(got []dns.RR, want []dns.RR) bool {
	 if len(got) != len(want) {
		 return false
	 }
	 for i := range got {
		 gotRR := got[i]
		 wantRR := want[i]
 
		 if gotRR.Header().Rrtype != wantRR.Header().Rrtype {
			 return false
		 }
 
		 switch gotRR.(type) {
		 case *dns.A:
			 gotIP := net.IP(gotRR.(*dns.A).A.To4())
			 wantIP := net.IP(wantRR.(*dns.A).A.To4())
			 if !gotIP.Equal(wantIP) {
				 return false
			 }
		 default:
		 }
	 }
 
	 return true
 }
 
 func bench(t *testing.B, nameserver string, hostname string) {
	 errs := 0
	 nrs := 0
	 nxdomain := 0
	 cnames := 0
	 c := dns.Client{
		 Timeout: 1 * time.Second,
	 }
	 for i := 0; i < t.N; i++ {
		 toResolve := hostname
	 redirect:
		 m := new(dns.Msg)
		 m.SetQuestion(toResolve, dns.TypeA)
		 res, _, err := c.Exchange(m, nameserver)
 
		 if err != nil {
			 errs++
		 } else if len(res.Answer) == 0 {
			 nrs++
		 } else {
			 for _, a := range res.Answer {
				 if arec, ok := a.(*dns.A); !ok {
					 // check if this is a cname redirect. If so, repeat the resolution
					 // assuming the client does not see/respect the inlined A record in the response.
					 if crec, ok := a.(*dns.CNAME); !ok {
						 errs++
					 } else {
						 cnames++
						 toResolve = crec.Target
						 goto redirect
					 }
				 } else {
					 if arec.Hdr.Rrtype != dns.RcodeSuccess {
						 nxdomain++
					 }
				 }
			 }
		 }
	 }
 
	 if errs+nrs > 0 {
		 t.Log("Sent", t.N, "err", errs, "no response", nrs, "nxdomain", nxdomain, "cname redirect", cnames)
	 }
 }
 