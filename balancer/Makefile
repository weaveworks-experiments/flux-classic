PKG:=github.com/dpw/ambergris
GOFILES:=$(shell find . -name '*.go')
SRC:=main.go interceptor coatl

.PHONY: image
image: docker/.server.done

.PHONY: clean
clean:
	rm -f ambergris docker/.*.done
	rm -rf build

.%.done: Dockerfile.%
	rm -rf build-container
	mkdir build-container
	cp -pr $^ build-container
	docker build -t ambergris/$(*F) -f build-container/$(<F) build-container
	rm -rf build-container
	touch $@

docker/.server.done: ambergris

run_build_container=docker run --rm -v $$PWD/build:/go \
    -v $$PWD/docker/build-wrapper.sh:/build-wrapper.sh \
    --workdir=/go/src/$(PKG) -e GOPATH=/go ambergris/build sh /build-wrapper.sh

ambergris: docker/.build.done docker/build-wrapper.sh $(GOFILES)
	rm -rf build/src/$(PKG)
	mkdir -p build/src/$(PKG)
	cp -pr $(SRC) build/src/$(PKG)
	$(run_build_container) "go get ./... && go build ./..."
	cp build/bin/ambergris $@

.PHONY: test
test: docker/.build.done $(GOFILES)
	rm -rf build/src/$(PKG)
	mkdir -p build/src/$(PKG)
	cp -pr $(SRC) build/src/$(PKG)
	$(run_build_container) "cd interceptor && go get -t ./... && go test ./..."
