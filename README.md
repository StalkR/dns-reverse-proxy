# DNS reverse proxy #

[![Build Status](https://api.travis-ci.org/StalkR/dns-reverse-proxy.png)](https://travis-ci.org/StalkR/dns-reverse-proxy) [![Godoc](https://godoc.org/github.com/StalkR/dns-reverse-proxy?status.png)](https://godoc.org/github.com/StalkR/dns-reverse-proxy)

A DNS reverse proxy to route queries to different DNS servers.
To illustrate, imagine an HTTP reverse proxy but for DNS.

It listens on both TCP/UDP IPv4/IPv6 on specified port.
Since the upstream servers will not see the real client IPs but the proxy,
you can specify a list of IPs allowed to transfer (AXFR/IXFR).

Example:

    $ go run dns_reverse_proxy.go -address :53 \
        -default 8.8.8.8:53 \
        -route .example.com.=8.8.4.4:53 \
        -allow-transfer 1.2.3.4,5.6.7.8

A query for `example.net` or `example.com` will go to `8.8.8.8:53`, the default.
However, a query for `subdomain.example.com` will go to `8.8.4.4:53`.

# Setup (Debian flavor) #

TODO(StalkR): Debian package. Until then:

    $ cd /usr/bin
    $ go build github.com/StalkR/dns-reverse-proxy
    $ cd /etc/init.d
    $ wget https://github.com/StalkR/dns-reverse-proxy/raw/master/etc/init.d/dns-reverse-proxy
    $ chmod +x /etc/init.d/dns-reverse-proxy
    $ insserv dns-reverse-proxy

Configure by editing `/etc/default/dns-reverse-proxy`. Example:

    DAEMON_ARGS="-default 127.0.0.1:1053 -route .example.com.=8.8.4.4:53 -allow-transfer 1.2.3.4,5.6.7.8"

And start:

    invoke-rc.d dns-reverse-proxy start

# License #

[Apache License, version 2.0](http://www.apache.org/licenses/LICENSE-2.0).

# Thanks #

- the powerful Go [dns](https://github.com/miekg/dns) library by [Miek Gieben](https://github.com/miekg)

# Bugs, feature requests, questions #

Create a [new issue](https://github.com/StalkR/dns-reverse-proxy/issues/new).
