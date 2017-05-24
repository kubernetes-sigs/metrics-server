RIEMANN_VERSION = 0.2.12

all:    install

install:
	go install
	go install ./proto

test:
	go test

integ-test:
	make integ ; make clean

integ:
	./integration.sh $(RIEMANN_VERSION)

clean:
	go clean ./...
	rm -f riemann-$(RIEMANN_VERSION).tar.bz2
	rm -rf riemann-$(RIEMANN_VERSION)
	rm -f riemann.PID

nuke:
	go clean -i ./...

regenerate:
	make -C proto regenerate
