# DNS reverse proxy

[![Build Status][build-img]][build]
[![Godoc][godoc-img]][godoc]

[build]: https://github.com/StalkR/dns-reverse-proxy/actions/workflows/build.yml
[build-img]: https://github.com/StalkR/dns-reverse-proxy/actions/workflows/build.yml/badge.svg
[godoc]: https://godoc.org/github.com/StalkR/dns-reverse-proxy
[godoc-img]: https://godoc.org/github.com/StalkR/dns-reverse-proxy?status.png

A DNS reverse proxy to route queries to different DNS servers.
To illustrate, imagine an HTTP reverse proxy but for DNS.

It listens on both TCP/UDP IPv4/IPv6 on specified port.
Since the upstream servers will not see the real client IPs but the proxy,
you can specify a list of IPs allowed to transfer (AXFR/IXFR).

Example:

    $ go run dns_reverse_proxy.go -address :53 \
        -default 8.8.8.8:53 \
        -route .example.com.=8.8.4.4:53 \
        -allow-transfer 1.2.3.4,::1

A query for `example.net` or `example.com` will go to `8.8.8.8:53`, the default.
However, a query for `subdomain.example.com` will go to `8.8.4.4:53`. `-default`
is optional - if it is not given then the server will return a failure for
queries for domains where a route has not been given.

# Setup

Install go package, create Debian package, install:

    $ go get -u github.com/miekg/dns
    $ go get -u github.com/StalkR/dns-reverse-proxy
    $ cd $GOPATH/src/github.com/StalkR/dns-reverse-proxy
    $ fakeroot debian/rules clean binary
    $ sudo dpkg -i ../dns-reverse-proxy_1-1_amd64.deb

Configure in `/etc/default/dns-reverse-proxy` and start with
`/etc/init.d/dns-reverse-proxy start`.

# License

[Apache License, version 2.0](http://www.apache.org/licenses/LICENSE-2.0).

# Thanks

- the powerful [github.com/miekg/dns][miekg/dns] Go library by [@miekg][miekg]

[miekg/dns]: https://github.com/miekg/dns
[miekg]: https://github.com/miekg

# Related

- [github.com/notsobad/dns-reverse-proxy][notsobad] fork with DNS-over-HTTPS
  routing support and passive DNS standard logging

[notsobad]: https://github.com/notsobad/dns-reverse-proxy

# Bugs, feature requests, questions

Create a [new issue][new-issue].

[new-issue]: https://github.com/StalkR/dns-reverse-proxy/issues/new
