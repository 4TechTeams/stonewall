export GOFLAGS = -buildvcs=false

.PHONY: build test e2e e2e-linux clean

build:
	go build -o stonewall .

test:
	go test ./...

e2e: build
	test/e2e.sh ./stonewall

# Same checks inside a Linux container. bwrap needs mount and user namespaces, hence --privileged.
e2e-linux:
	docker build -q -t stonewall-e2e test
	docker run --rm --privileged -v "$(CURDIR)":/src -v stonewall-gomod:/go/pkg/mod -e GOFLAGS stonewall-e2e \
		sh -c 'go test ./... && go build -o /tmp/stonewall . && test/e2e.sh /tmp/stonewall'

clean:
	rm -f stonewall
