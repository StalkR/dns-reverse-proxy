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
-route .example2.com.=8.8.4.4:53,1.1.1.1:53 \
-route .example3.com.=https://dns.alidns.com \
-allow-transfer 1.2.3.4,::1

A query for example.net or example.com will go to 8.8.8.8:53, the default.
However, a query for subdomain.example.com will go to 8.8.4.4:53. -default
is optional - if it is not given then the server will return a failure for
queries for domains where a route has not been given.
*/
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/babolivier/go-doh-client"

	"github.com/miekg/dns"
)

type flagStringList []string

// passivedns style log
// https://github.com/gamelinux/passivedns
// #timestamp||dns-client ||dns-server||RR class||Query||Query Type||Answer||TTL||Count
// 1322849924.408856||10.1.1.1||8.8.8.8||IN||upload.youtube.com.||A||74.125.43.117||46587||5
type pdnsLog struct {
	timestamp string
	dnsClient string
	dnsServer string
	rrClass   string
	query     string
	queryType string
	answer    string
	ttl       string
	count     string
}

func (p *pdnsLog) String() string {
	arr := []string{
		p.timestamp,
		p.dnsClient,
		p.dnsServer,
		p.rrClass,
		p.query,
		p.queryType,
		p.answer,
		p.ttl,
		p.count,
	}
	log := strings.Join(arr, "||")
	return log
}

func (i *flagStringList) String() string {
	return fmt.Sprint(*i)
}

func (i *flagStringList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var (
	address = flag.String("address", ":53", "Address to listen to (TCP and UDP)")

	defaultServer = flag.String("default", "",
		"Default DNS server where to send queries if no route matched (host:port)")

	routeLists flagStringList
	routes     map[string][]string

	allowTransfer = flag.String("allow-transfer", "",
		"List of IPs allowed to transfer (AXFR/IXFR)")
	transferIPs []string
)

func init() {
	rand.Seed(time.Now().Unix())
	flag.Var(&routeLists, "route", "List of routes where to send queries (domain=host:port,[host:port,...])")
}

func main() {
	flag.Parse()

	transferIPs = strings.Split(*allowTransfer, ",")
	routes = make(map[string][]string)
	for _, routeList := range routeLists {
		s := strings.SplitN(routeList, "=", 2)
		if len(s) != 2 || len(s[0]) == 0 || len(s[1]) == 0 {
			log.Fatal("invalid -route, must be domain=host:port,[host:port,...]")
		}
		var backends []string
		for _, backend := range strings.Split(s[1], ",") {
			host, port, err := net.SplitHostPort(backend)

			if err != nil || host == "" || port == "" {
				log.Fatalf("invalid host:port for %v", backend)
			}

			backends = append(backends, backend)
		}
		if !strings.HasSuffix(s[0], ".") {
			s[0] += "."
		}
		routes[strings.ToLower(s[0])] = backends
	}

	udpServer := &dns.Server{Addr: *address, Net: "udp"}
	tcpServer := &dns.Server{Addr: *address, Net: "tcp"}
	dns.HandleFunc(".", route)

	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()
	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	udpServer.Shutdown()
	tcpServer.Shutdown()
}

func lookupDoH(addr string, w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	q := req.Question[0]
	lcName := strings.ToLower(q.Name)
	//fmt.Println("lcName", lcName, q.Qtype)
	domain := strings.TrimSuffix(lcName, ".")

	resolver := doh.Resolver{
		Host:  addr,
		Class: doh.IN,
	}

	m := new(dns.Msg)
	m.SetReply(req)
	m.RecursionAvailable = false
	m.Authoritative = true

	var answers []dns.RR
	hdr := dns.RR_Header{Name: lcName, Rrtype: q.Qtype, Class: dns.ClassINET}

	switch q.Qtype {
	case dns.TypeA:
		ans, _, err := resolver.LookupA(domain)
		if err != nil {
			log.Println(err)
			break
		}

		for _, a := range ans {
			r := new(dns.A)
			r.Hdr = hdr
			r.A = net.ParseIP(a.IP4)
			answers = append(answers, r)
		}
	case dns.TypeAAAA:
		ans, _, err := resolver.LookupAAAA(domain)
		if err != nil {
			log.Println(err)
			break
		}

		for _, a := range ans {
			r := new(dns.AAAA)
			r.Hdr = hdr
			r.AAAA = net.ParseIP(a.IP6)
			answers = append(answers, r)
		}
	case dns.TypeCNAME:
		ans, _, err := resolver.LookupCNAME(domain)
		if err != nil {
			log.Println(err)
			break
		}

		for _, a := range ans {
			r := new(dns.CNAME)
			r.Hdr = hdr
			cname := a.CNAME
			if !strings.HasSuffix(cname, ".") {
				cname = cname + "."
			}
			r.Target = cname
			answers = append(answers, r)
		}
	case dns.TypeSOA:

		ans, _, err := resolver.LookupSOA(domain)
		if err != nil {
			log.Println(err)
			break
		}

		for _, a := range ans {
			r := new(dns.SOA)
			r.Hdr = hdr
			r.Ns = a.PrimaryNS
			r.Mbox = a.RespMailbox
			r.Serial = a.Serial
			r.Refresh = uint32(a.Refresh)
			r.Retry = uint32(a.Retry)
			r.Expire = uint32(a.Expire)
			r.Minttl = a.Minimum

			answers = append(answers, r)
		}
	}

	m.Answer = append(m.Answer, answers...)
	//fmt.Println(lcName, answers)
	err := w.WriteMsg(m)
	if err != nil {
		log.Printf("Error writing msg %s\n", err)
	}
	return m
}

func route(w dns.ResponseWriter, req *dns.Msg) {
	if len(req.Question) == 0 || !allowed(w, req) {
		dns.HandleFailed(w, req)
		return
	}

	lcName := strings.ToLower(req.Question[0].Name)
	for name, addrs := range routes {
		if strings.HasSuffix(lcName, name) {
			addr := addrs[0]
			if n := len(addrs); n > 1 {
				addr = addrs[rand.Intn(n)]
			}
			proxy(addr, w, req)
			return
		}
	}

	if *defaultServer == "" {
		dns.HandleFailed(w, req)
		return
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
	var resp *dns.Msg
	if strings.HasPrefix(addr, "https://") {
		addr = strings.Replace(addr, "https://", "", 1)
		resp = lookupDoH(addr, w, req)
	} else {
		c := &dns.Client{Net: transport}
		var _ time.Duration
		var err error
		resp, _, err = c.Exchange(req, addr)
		if err != nil {
			dns.HandleFailed(w, req)
			return
		}
	}

	w.WriteMsg(resp)

	go func() {

		for _, r := range resp.Answer {
			p := new(pdnsLog)

			p.dnsClient = w.RemoteAddr().String()
			p.timestamp = fmt.Sprintf("%f", float64(time.Now().UnixMicro())/float64(1e6))
			p.dnsServer = addr
			p.ttl = fmt.Sprintf("%d", r.Header().Ttl)
			p.rrClass = dns.Class(r.Header().Class).String()
			p.count = "1" // what does the count means?
			if rec, ok := r.(*dns.A); ok {
				p.query = rec.Hdr.Name
				p.queryType = dns.Type(rec.Hdr.Rrtype).String()
				p.answer = rec.A.String()
			} else if rec, ok := r.(*dns.AAAA); ok {
				p.queryType = dns.Type(rec.Hdr.Rrtype).String()
				p.query = rec.Hdr.Name
				p.answer = rec.AAAA.String()
			} else if rec, ok := r.(*dns.CNAME); ok {
				p.queryType = dns.Type(rec.Hdr.Rrtype).String()
				p.query = rec.Hdr.Name
				p.answer = rec.Target
			}

			fmt.Println(p.String())
		}
	}()
}
