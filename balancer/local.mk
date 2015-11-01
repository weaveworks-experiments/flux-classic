test: test-balancer

.PHONY: test-balancer
test-balancer: docker/.build.done $(shell find balancer pkg -name '*.go')
	rm -rf build/src/$(BASEPKG)/balancer
	mkdir -p build/src/$(BASEPKG)/balancer
	cp -pr balancer pkg build/src/$(BASEPKG)/
	$(call run_build_container,-e GOPATH=/build,build,src/$(BASEPKG)/balancer/interceptor,go get -t ./... && go test ./...)
