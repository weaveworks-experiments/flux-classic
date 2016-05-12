REPO:=weaveworks
PROJECT:=$(REPO)/flux
BASEPKG:=github.com/$(PROJECT)
REVISION:="$(shell git rev-parse --short=12 HEAD)"
VERSION:="head"
GOFLAGS:=-ldflags "-X $(BASEPKG)/common/version.version=$(VERSION) -X $(BASEPKG)/common/version.revision=$(REVISION)"

BUILD_IMAGES=build webbuild site
IMAGES:=fluxd web fluxctl edgebal prometheus-etcd
GODIRS:=$(COMPONENTS) common

# The go "main" package directories
CMD_DIRS:=cmd/fluxd cmd/balagent web fluxctl

image_stamp=docker/.$1.done
docker_tag=$(if $(filter flux%,$1),$(REPO)/$1,$(REPO)/flux-$1)

GO_SRCS:=$(shell find * -name vendor -prune -o -name "*.go" -print)
GODIRS:=$(sort $(foreach F,$(GO_SRCS),$(firstword $(subst /, ,$(F)))))

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

$(call image_stamp,fluxd): build/bin/fluxd
$(call image_stamp,fluxctl): build/bin/fluxctl
$(call image_stamp,web): build/bin/web

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

$(foreach d,$(CMD_DIRS),$(eval build/bin/$(notdir $(d)): build/bin/.stamp; @true))

build/bin/.stamp: $(GO_SRCS) $(call image_stamp,build) docker/build-wrapper.sh
	mkdir -p $(@D) && touch $@.tmp
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go install $(GOFLAGS) $(foreach d,$(CMD_DIRS),$(BASEPKG)/$(d)))
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
	$(call run_build_container,build,-e GOPATH=/build,,go tool vet $$(find * -maxdepth 0 -type d -not -name vendor))

# Subdir-specific rules
include ./*/local.mk
