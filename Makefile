PROJ:=ambergreen
BASEPKG:=github.com/squaremo/$(PROJ)

DEPS:=$(shell find pkg -name '*.go')

.PHONY: images
images: docker/.balancer.done docker/.agent.done \
	docker/.display.done docker/.command.done

.PHONY: clean
clean:
	rm -f docker/.*.done
	rm -rf ./build

.PHONY: test
test:

# Don't remove this if a subsequent step fails
.PRECIOUS: docker/.build.done

.%.done: Dockerfile.%
	rm -rf build-container
	mkdir build-container
	cp -pr $^ build-container/
	docker build -t $(PROJ)/$(*F) -f build-container/$(<F) build-container
	rm -rf build-container
	touch $@

docker/.agent.done: agent.bin
docker/.balancer.done: balancer.bin
docker/.command.done: command.bin
docker/.display.done: display.bin

run_build_container=docker run --rm -v $$PWD/build:/go \
    -v $$PWD/docker/build-wrapper.sh:/build-wrapper.sh \
    --workdir=/go/src/$(BASEPKG)/$(*F) -e GOPATH=/go $(PROJ)/build sh /build-wrapper.sh

%.bin: docker/.build.done docker/build-wrapper.sh $(DEPS)
	rm -rf build/src/$(BASEPKG)
	mkdir -p build/src/$(BASEPKG)
	cp -pr pkg $(*F) build/src/$(BASEPKG)/
	$(run_build_container) "go get ./... && go build ./..."
	cp build/bin/$(*F) $@

# Subdir-specific rules

include ./*/local.mk
