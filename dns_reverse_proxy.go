/*
Binary dns_reverse_proxy is a DNS reverse proxy to route queries to DNS servers.

To illustrate, imagine an HTTP reverse proxy but for DNS.
It listens on both TCP/UDP IPv4/IPv6 on specified port.
Since the upstream servers will not see the real client IPs but the proxy,
you can specify a list of IPs allowed to transfer (AXFR/IXFR).

Example usage:
        $ go run dns_reverse_proxy.go -address :53 \
                -default 8.8.8.8:53 \
                -route .example.com.=8.8.4.4:53 \
                -allow-transfer 1.2.3.4,::1

A query for example.net or example.com will go to 8.8.8.8:53, the default.
However, a query for subdomain.example.com will go to 8.8.4.4:53.
*/
package main

import (
	"flag"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

var (
	address = flag.String("address", ":53", "Address to listen to (TCP and UDP)")

	defaultServer = flag.String("default", "",
		"Default DNS server where to send queries if no route matched (IP:port)")

	routeList = flag.String("route", "",
		"List of routes where to send queries (subdomain=IP:port)")
	routes map[string]string

	allowTransfer = flag.String("allow-transfer", "",
		"List of IPs allowed to transfer (AXFR/IXFR)")
	transferIPs []string
)

func main() {
	flag.Parse()
	if *defaultServer == "" {
		log.Fatal("-default is required")
	}
	transferIPs = strings.Split(*allowTransfer, ",")
	routes = make(map[string]string)
	if *routeList != "" {
		for _, s := range strings.Split(*routeList, ",") {
			s := strings.SplitN(s, "=", 2)
			if len(s) != 2 {
				log.Fatal("invalid -routes format")
			}
			if !strings.HasSuffix(s[0], ".") {
				s[0] += "."
			}
			routes[s[0]] = s[1]
		}
	}

	udpServer := &dns.Server{Addr: *address, Net: "udp"}
	tcpServer := &dns.Server{Addr: *address, Net: "tcp"}
	dns.HandleFunc(".", route)
	go func() {
		log.Fatal(udpServer.ListenAndServe())
	}()
	log.Fatal(tcpServer.ListenAndServe())
}

func route(w dns.ResponseWriter, req *dns.Msg) {
	if len(req.Question) == 0 || !allowed(w, req) {
		dns.HandleFailed(w, req)
		return
	}
	for name, addr := range routes {
		if strings.HasSuffix(req.Question[0].Name, name) {
			proxy(addr, w, req)
			return
		}
	}
	proxy(*defaultServer, w, req)
}

func isTransfer(req *dns.Msg) bool {
	for _, q := range req.Question {
		switch q.Qtype {
		case dns.TypeIXFR, dns.TypeAXFR:
			return true
		}
	}
	return false
}

func allowed(w dns.ResponseWriter, req *dns.Msg) bool {
	if !isTransfer(req) {
		return true
	}
	remote, _, _ := net.SplitHostPort(w.RemoteAddr().String())
	for _, ip := range transferIPs {
		if ip == remote {
			return true
		}
	}
	return false
}

func proxy(addr string, w dns.ResponseWriter, req *dns.Msg) {
	transport := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		transport = "tcp"
	}
	if isTransfer(req) {
		if transport != "tcp" {
			dns.HandleFailed(w, req)
			return
		}
		t := new(dns.Transfer)
		c, err := t.In(req, addr)
		if err != nil {
			dns.HandleFailed(w, req)
			return
		}
		if err = t.Out(w, req, c); err != nil {
			dns.HandleFailed(w, req)
			return
		}
		return
	}
	c := &dns.Client{Net: transport}
	resp, _, err := c.Exchange(req, addr)
	if err != nil {
		dns.HandleFailed(w, req)
		return
	}
	w.WriteMsg(resp)
}
