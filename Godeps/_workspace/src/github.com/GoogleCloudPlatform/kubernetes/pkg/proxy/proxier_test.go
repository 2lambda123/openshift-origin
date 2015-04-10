/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/iptables"
)

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func waitForClosedPortTCP(p *Proxier, proxyPort int) error {
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("tcp", joinHostPort("", proxyPort))
		if err != nil {
			return nil
		}
		conn.Close()
		time.Sleep(1 * time.Millisecond)
	}
	return fmt.Errorf("port %d still open", proxyPort)
}

func waitForClosedPortUDP(p *Proxier, proxyPort int) error {
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("udp", joinHostPort("", proxyPort))
		if err != nil {
			return nil
		}
		conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		// To detect a closed UDP port write, then read.
		_, err = conn.Write([]byte("x"))
		if err != nil {
			if e, ok := err.(net.Error); ok && !e.Timeout() {
				return nil
			}
		}
		var buf [4]byte
		_, err = conn.Read(buf[0:])
		if err != nil {
			if e, ok := err.(net.Error); ok && !e.Timeout() {
				return nil
			}
		}
		conn.Close()
		time.Sleep(1 * time.Millisecond)
	}
	return fmt.Errorf("port %d still open", proxyPort)
}

// The iptables logic has to be tested in a proper end-to-end test, so this just stubs everything out.
type fakeIptables struct{}

func (fake *fakeIptables) EnsureChain(table iptables.Table, chain iptables.Chain) (bool, error) {
	return false, nil
}

func (fake *fakeIptables) DeleteChain(table iptables.Table, chain iptables.Chain) error {
	return nil
}

func (fake *fakeIptables) FlushChain(table iptables.Table, chain iptables.Chain) error {
	return nil
}

func (fake *fakeIptables) EnsureRule(table iptables.Table, chain iptables.Chain, args ...string) (bool, error) {
	return false, nil
}

func (fake *fakeIptables) DeleteRule(table iptables.Table, chain iptables.Chain, args ...string) error {
	return nil
}

func (fake *fakeIptables) IsIpv6() bool {
	return false
}

var tcpServerPort int
var udpServerPort int

func init() {
	// Don't handle panics
	util.ReallyCrash = true

	// TCP setup.
	tcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.URL.Path[1:]))
	}))
	u, err := url.Parse(tcp.URL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse: %v", err))
	}
	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		panic(fmt.Sprintf("failed to parse: %v", err))
	}
	tcpServerPort, err = strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("failed to atoi(%s): %v", port, err))
	}

	// UDP setup.
	udp, err := newUDPEchoServer()
	if err != nil {
		panic(fmt.Sprintf("failed to make a UDP server: %v", err))
	}
	_, port, err = net.SplitHostPort(udp.LocalAddr().String())
	if err != nil {
		panic(fmt.Sprintf("failed to parse: %v", err))
	}
	udpServerPort, err = strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("failed to atoi(%s): %v", port, err))
	}
	go udp.Loop()
}

func testEchoTCP(t *testing.T, address string, port int) {
	path := "aaaaa"
	res, err := http.Get("http://" + address + ":" + fmt.Sprintf("%d", port) + "/" + path)
	if err != nil {
		t.Fatalf("error connecting to server: %v", err)
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("error reading data: %v %v", err, string(data))
	}
	if string(data) != path {
		t.Errorf("expected: %s, got %s", path, string(data))
	}
}

func testEchoUDP(t *testing.T, address string, port int) {
	data := "abc123"

	conn, err := net.Dial("udp", joinHostPort(address, port))
	if err != nil {
		t.Fatalf("error connecting to server: %v", err)
	}
	if _, err := conn.Write([]byte(data)); err != nil {
		t.Fatalf("error sending to server: %v", err)
	}
	var resp [1024]byte
	n, err := conn.Read(resp[0:])
	if err != nil {
		t.Errorf("error receiving data: %v", err)
	}
	if string(resp[0:n]) != data {
		t.Errorf("expected: %s, got %s", data, string(resp[0:n]))
	}
}

func waitForNumProxyLoops(t *testing.T, p *Proxier, want int32) {
	var got int32
	for i := 0; i < 4; i++ {
		got = atomic.LoadInt32(&p.numProxyLoops)
		if got == want {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Errorf("expected %d ProxyLoops running, got %d", want, got)
}

func TestTCPProxy(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)
}

func TestUDPProxy(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: udpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "UDP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoUDP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)
}

func TestMultiPortProxy(t *testing.T) {
	lb := NewLoadBalancerRR()
	serviceP := ServicePortName{types.NamespacedName{"testnamespace", "echo-p"}, "p"}
	serviceQ := ServicePortName{types.NamespacedName{"testnamespace", "echo-q"}, "q"}
	lb.OnUpdate([]api.Endpoints{{
		ObjectMeta: api.ObjectMeta{Name: serviceP.Name, Namespace: serviceP.Namespace},
		Subsets: []api.EndpointSubset{{
			Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
			Ports:     []api.EndpointPort{{Name: "p", Protocol: "TCP", Port: tcpServerPort}},
		}},
	}, {
		ObjectMeta: api.ObjectMeta{Name: serviceQ.Name, Namespace: serviceQ.Namespace},
		Subsets: []api.EndpointSubset{{
			Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
			Ports:     []api.EndpointPort{{Name: "q", Protocol: "UDP", Port: udpServerPort}},
		}},
	}})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfoP, err := p.addServiceOnPort(serviceP, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoTCP(t, "127.0.0.1", svcInfoP.proxyPort)
	waitForNumProxyLoops(t, p, 1)

	svcInfoQ, err := p.addServiceOnPort(serviceQ, "UDP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoUDP(t, "127.0.0.1", svcInfoQ.proxyPort)
	waitForNumProxyLoops(t, p, 2)
}

func TestMultiPortOnUpdate(t *testing.T) {
	lb := NewLoadBalancerRR()
	serviceP := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	serviceQ := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "q"}
	serviceX := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "x"}

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: serviceP.Name, Namespace: serviceP.Namespace},
		Spec: api.ServiceSpec{PortalIP: "1.2.3.4", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     80,
			Protocol: "TCP",
		}, {
			Name:     "q",
			Port:     81,
			Protocol: "UDP",
		}}},
	}})
	waitForNumProxyLoops(t, p, 2)
	svcInfo, exists := p.getServiceInfo(serviceP)
	if !exists {
		t.Fatalf("can't find serviceInfo for %s", serviceP)
	}
	if svcInfo.portalIP.String() != "1.2.3.4" || svcInfo.portalPort != 80 || svcInfo.protocol != "TCP" {
		t.Errorf("unexpected serviceInfo for %s: %#v", serviceP, svcInfo)
	}

	svcInfo, exists = p.getServiceInfo(serviceQ)
	if !exists {
		t.Fatalf("can't find serviceInfo for %s", serviceQ)
	}
	if svcInfo.portalIP.String() != "1.2.3.4" || svcInfo.portalPort != 81 || svcInfo.protocol != "UDP" {
		t.Errorf("unexpected serviceInfo for %s: %#v", serviceQ, svcInfo)
	}

	svcInfo, exists = p.getServiceInfo(serviceX)
	if exists {
		t.Fatalf("found unwanted serviceInfo for %s: %#v", serviceX, svcInfo)
	}
}

// Helper: Stops the proxy for the named service.
func stopProxyByName(proxier *Proxier, service ServicePortName) error {
	info, found := proxier.getServiceInfo(service)
	if !found {
		return fmt.Errorf("unknown service: %s", service)
	}
	return proxier.stopProxy(service, info)
}

func TestTCPProxyStop(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Namespace: service.Namespace, Name: service.Name},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	conn, err := net.Dial("tcp", joinHostPort("", svcInfo.proxyPort))
	if err != nil {
		t.Fatalf("error connecting to proxy: %v", err)
	}
	conn.Close()
	waitForNumProxyLoops(t, p, 1)

	stopProxyByName(p, service)
	// Wait for the port to really close.
	if err := waitForClosedPortTCP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	waitForNumProxyLoops(t, p, 0)
}

func TestUDPProxyStop(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Namespace: service.Namespace, Name: service.Name},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: udpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "UDP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	conn, err := net.Dial("udp", joinHostPort("", svcInfo.proxyPort))
	if err != nil {
		t.Fatalf("error connecting to proxy: %v", err)
	}
	conn.Close()
	waitForNumProxyLoops(t, p, 1)

	stopProxyByName(p, service)
	// Wait for the port to really close.
	if err := waitForClosedPortUDP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	waitForNumProxyLoops(t, p, 0)
}

func TestTCPProxyUpdateDelete(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Namespace: service.Namespace, Name: service.Name},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	conn, err := net.Dial("tcp", joinHostPort("", svcInfo.proxyPort))
	if err != nil {
		t.Fatalf("error connecting to proxy: %v", err)
	}
	conn.Close()
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{})
	if err := waitForClosedPortTCP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	waitForNumProxyLoops(t, p, 0)
}

func TestUDPProxyUpdateDelete(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Namespace: service.Namespace, Name: service.Name},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: udpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "UDP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	conn, err := net.Dial("udp", joinHostPort("", svcInfo.proxyPort))
	if err != nil {
		t.Fatalf("error connecting to proxy: %v", err)
	}
	conn.Close()
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{})
	if err := waitForClosedPortUDP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	waitForNumProxyLoops(t, p, 0)
}

func TestTCPProxyUpdateDeleteUpdate(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	conn, err := net.Dial("tcp", joinHostPort("", svcInfo.proxyPort))
	if err != nil {
		t.Fatalf("error connecting to proxy: %v", err)
	}
	conn.Close()
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{})
	if err := waitForClosedPortTCP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	waitForNumProxyLoops(t, p, 0)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "1.2.3.4", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     svcInfo.proxyPort,
			Protocol: "TCP",
		}}},
	}})
	svcInfo, exists := p.getServiceInfo(service)
	if !exists {
		t.Fatalf("can't find serviceInfo for %s", service)
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)
}

func TestUDPProxyUpdateDeleteUpdate(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: udpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "UDP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	conn, err := net.Dial("udp", joinHostPort("", svcInfo.proxyPort))
	if err != nil {
		t.Fatalf("error connecting to proxy: %v", err)
	}
	conn.Close()
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{})
	if err := waitForClosedPortUDP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	waitForNumProxyLoops(t, p, 0)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "1.2.3.4", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     svcInfo.proxyPort,
			Protocol: "UDP",
		}}},
	}})
	svcInfo, exists := p.getServiceInfo(service)
	if !exists {
		t.Fatalf("can't find serviceInfo")
	}
	testEchoUDP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)
}

func TestTCPProxyUpdatePort(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "1.2.3.4", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     99,
			Protocol: "TCP",
		}}},
	}})
	// Wait for the socket to actually get free.
	if err := waitForClosedPortTCP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	svcInfo, exists := p.getServiceInfo(service)
	if !exists {
		t.Fatalf("can't find serviceInfo")
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	// This is a bit async, but this should be sufficient.
	time.Sleep(500 * time.Millisecond)
	waitForNumProxyLoops(t, p, 1)
}

func TestUDPProxyUpdatePort(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: udpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "UDP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "1.2.3.4", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     99,
			Protocol: "UDP",
		}}},
	}})
	// Wait for the socket to actually get free.
	if err := waitForClosedPortUDP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	svcInfo, exists := p.getServiceInfo(service)
	if !exists {
		t.Fatalf("can't find serviceInfo")
	}
	testEchoUDP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)
}

func TestProxyUpdatePublicIPs(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{
			Ports: []api.ServicePort{{
				Name:     "p",
				Port:     svcInfo.portalPort,
				Protocol: "TCP",
			}},
			PortalIP:  svcInfo.portalIP.String(),
			PublicIPs: []string{"4.3.2.1"},
		},
	}})
	// Wait for the socket to actually get free.
	if err := waitForClosedPortTCP(p, svcInfo.proxyPort); err != nil {
		t.Fatalf(err.Error())
	}
	svcInfo, exists := p.getServiceInfo(service)
	if !exists {
		t.Fatalf("can't find serviceInfo")
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	// This is a bit async, but this should be sufficient.
	time.Sleep(500 * time.Millisecond)
	waitForNumProxyLoops(t, p, 1)
}

func TestProxyUpdatePortal(t *testing.T) {
	lb := NewLoadBalancerRR()
	service := ServicePortName{types.NamespacedName{"testnamespace", "echo"}, "p"}
	lb.OnUpdate([]api.Endpoints{
		{
			ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
			Subsets: []api.EndpointSubset{{
				Addresses: []api.EndpointAddress{{IP: "127.0.0.1"}},
				Ports:     []api.EndpointPort{{Name: "p", Port: tcpServerPort}},
			}},
		},
	})

	p := CreateProxier(lb, net.ParseIP("0.0.0.0"), &fakeIptables{}, net.ParseIP("127.0.0.1"))
	waitForNumProxyLoops(t, p, 0)

	svcInfo, err := p.addServiceOnPort(service, "TCP", 0, time.Second)
	if err != nil {
		t.Fatalf("error adding new service: %#v", err)
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     svcInfo.proxyPort,
			Protocol: "TCP",
		}}},
	}})
	_, exists := p.getServiceInfo(service)
	if exists {
		t.Fatalf("service with empty portalIP should not be included in the proxy")
	}

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "None", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     svcInfo.proxyPort,
			Protocol: "TCP",
		}}},
	}})
	_, exists = p.getServiceInfo(service)
	if exists {
		t.Fatalf("service with 'None' as portalIP should not be included in the proxy")
	}

	p.OnUpdate([]api.Service{{
		ObjectMeta: api.ObjectMeta{Name: service.Name, Namespace: service.Namespace},
		Spec: api.ServiceSpec{PortalIP: "1.2.3.4", Ports: []api.ServicePort{{
			Name:     "p",
			Port:     svcInfo.proxyPort,
			Protocol: "TCP",
		}}},
	}})
	svcInfo, exists = p.getServiceInfo(service)
	if !exists {
		t.Fatalf("service with portalIP set not found in the proxy")
	}
	testEchoTCP(t, "127.0.0.1", svcInfo.proxyPort)
	waitForNumProxyLoops(t, p, 1)
}

// TODO: Test UDP timeouts.
