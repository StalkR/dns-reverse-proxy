all:
	go build .
install:
	mkdir -p $(DESTDIR)/usr/bin
	cp dns-reverse-proxy $(DESTDIR)/usr/bin
	chmod 755 $(DESTDIR)/usr/bin/dns-reverse-proxy
clean:  
	rm -f dns-reverse-proxy
