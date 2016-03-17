REPO:=weaveworks
PROJECT:=$(REPO)/flux
BASEPKG:=github.com/$(PROJECT)
REVISION:="$(shell git rev-parse --short=12 HEAD)"
VERSION:="head"
GOFLAGS:=-ldflags "-X $(BASEPKG)/common/version.version=$(VERSION) -X $(BASEPKG)/common/version.revision=$(REVISION)"

BUILD_IMAGES=build webbuild site
COMPONENTS:=balancer agent web fluxctl
IMAGES:=$(COMPONENTS) edgebal prometheus-etcd
GODIRS:=$(COMPONENTS) common
CMDS:=$(COMPONENTS) balagent

image_stamp=docker/.$1.done
docker_tag=$(PROJECT)-$1

# Where the main package for each command lives
CMD_DIR_agent:=agent/cmd/agent
CMD_DIR_web:=web
CMD_DIR_fluxctl:=fluxctl
CMD_DIR_balancer:=balancer/cmd/balancer
CMD_DIR_balagent:=balancer/cmd/balagent

GO_SRCS:=$(shell find $(GODIRS) -name "*.go")

# Delete files produced by failing recipes
.DELETE_ON_ERROR:

.PHONY: images
images: $(foreach i,$(IMAGES),docker/$(i).tar)

.PHONY: clean
clean::
	rm -rf build cover
	rm -f $(foreach i,$(IMAGES),$(call image_stamp,$(i))) docker/*.tar

.PHONY: realclean
realclean:: clean
	rm -rf $(foreach i,$(BUILD_IMAGES),$(call image_stamp,$(i)))

# Don't remove this if a subsequent step fails
.PRECIOUS: $(call image_stamp,build)

$(foreach i,$(IMAGES) $(BUILD_IMAGES),$(call image_stamp,$(i))): docker/.%.done: docker/Dockerfile.%
	rm -rf build-container.$*
	mkdir build-container.$*
	cp -pr $^ build-container.$*/
	docker build -t $(call docker_tag,$(*F)) -f build-container.$*/$(<F) build-container.$*
	rm -rf build-container.$*
	touch $@

$(foreach i,$(IMAGES),docker/$(i).tar): docker/%.tar: docker/.%.done
	docker save --output=$@ $(call docker_tag,$(*F))

$(foreach i,$(COMPONENTS),$(eval $(call image_stamp,$(i)): build/bin/$(i)))

# $1: build image
# $2: extra docker run args
# $3: working directory under /build/src/$(BASEPKG)
# $4: command string to pass to build-wrapper.sh
run_build_container=mkdir -p build/src/$(BASEPKG) && docker run --rm $2 \
    -v $$PWD/build:/build \
    -v $$PWD:/build/src/$(BASEPKG) \
    -v $$PWD/docker/build-wrapper.sh:/build-wrapper.sh \
    --workdir=/build/src/$(BASEPKG)$(and $3,/$3) \
    $(call docker_tag,$1) sh /build-wrapper.sh '$(subst ','"'"',$4)'

get_vendor_submodules=@git submodule update --init

$(foreach c,$(CMDS),$(eval build/bin/$(c): build/bin/.stamp; @true))

build/bin/.stamp: $(GO_SRCS) $(call image_stamp,build) docker/build-wrapper.sh
	mkdir -p $(@D) && touch $@.tmp
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go install $(GOFLAGS) $(foreach c,$(CMDS),$(BASEPKG)/$(CMD_DIR_$(c))))
	mv $@.tmp $@

GO_TEST_OPTS:=-timeout 5s

.PHONY: $(foreach i,$(GODIRS),test-$(i))
$(foreach i,$(GODIRS),test-$(i)): test-%: $(call image_stamp,build)
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go test $(GO_TEST_OPTS) ./$*/...)

.PHONY: test
test:: $(call image_stamp,build)
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go test $(GO_TEST_OPTS) $$(go list ./... | grep -v /vendor/))

.PHONY: cover
cover: $(call image_stamp,build)
	rm -rf cover
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,\
	    for d in $$(find * -path vendor -prune -o -name "*_test.go" -printf "%h\n" | sort -u); do \
	        mkdir -p cover/$$d && \
	        go test $(GO_TEST_OPTS) -coverprofile=cover/$$d.out $(BASEPKG)/$$d && \
	        go tool cover -html=cover/$$d.out -o cover/$$d.html ; \
	    done)

.PHONY: vet
vet:: $(call image_stamp,build)
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go vet $$(go list ./... | grep -v /vendor/))

# Subdir-specific rules
include ./*/local.mk
