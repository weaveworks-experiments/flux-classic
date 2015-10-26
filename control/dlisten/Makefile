PKG:=github.com/bboreham/coatl
GOFILES:=$(shell find . ../backends ../data -name '*.go')
SRC:=main.go listener.go ../data ../backends

.PHONY: image clean

image: docker/.server.done

clean:
	rm -f dlisten docker/.*.done
	rm -rf build

.%.done: Dockerfile.%
	rm -rf build-container
	mkdir build-container
	cp -pr $^ build-container
	docker build -t dlisten/$(*F) -f build-container/$(<F) build-container
	rm -rf build-container
	touch $@

docker/.server.done: dlisten

dlisten: docker/.build.done docker/build-in-container.sh $(GOFILES)
	rm -rf build/src/$(PKG)
	mkdir -p build/src/$(PKG)
	cp -pr $(SRC) build/src/$(PKG)
	docker run --rm -v $$PWD/build:/go \
	    -v $$PWD/docker/build-in-container.sh:/build.sh \
	    --workdir=/go/src/$(PKG) -e GOPATH=/go dlisten/build sh /build.sh
# Because we copy into github.com/bboreham/coatl/, the binary is named
# coatl rather than dlisten
	cp build/bin/coatl $@
