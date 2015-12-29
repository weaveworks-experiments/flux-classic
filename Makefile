REPO:=squaremo
PROJ:=ambergreen
BASEPKG:=github.com/$(REPO)/$(PROJ)
BIN_IMAGES=balancer agent web amberctl
BAKE_IMAGES=edgebal
IMAGES=$(BIN_IMAGES) $(BAKE_IMAGES)
BUILD_IMAGES=build webbuild

image_stamp=docker/.$1.done
docker_tag=$(REPO)/$(PROJ)-$1

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
	rm -rf build-container
	mkdir build-container
	cp -pr $^ build-container/
	docker build -t $(call docker_tag,$(*F)) -f build-container/$(<F) build-container
	rm -rf build-container
	touch $@

$(foreach i,$(IMAGES),docker/$(i).tar): docker/%.tar: docker/.%.done
	docker save --output=$@ $(call docker_tag,$(*F))

$(foreach i,$(BIN_IMAGES),$(eval $(call image_stamp,$(i)): build/bin/$(i)))
$(foreach i,$(BIN_IMAGES) common,$(eval $(i)_go_srcs:=$(shell find $(i) -name '*.go')))
$(foreach i,$(BIN_IMAGES),$(eval build/bin/$(i): $($(i)_go_srcs)))

# $1: build image
# $2: extra docker run args
# $3: working directory under /build/src/$(BASEPKG)
# $4: command string to pass to build-wrapper.sh
run_build_container=mkdir -p build/src/$(BASEPKG) && docker run --rm $2 \
    -v $$PWD/build:/build \
    -v $$PWD:/build/src/$(BASEPKG) \
    -v $$PWD/docker/build-wrapper.sh:/build-wrapper.sh \
    --workdir=/build/src/$(BASEPKG)$(and $3,/$3) \
    $(PROJ)/$1 sh /build-wrapper.sh '$(subst ','"'"',$4)'

get_vendor_submodules=@if [ -z "$$(find vendor -type f -print -quit)" ] ; then git submodule update --init ; fi

# Where the main package for each command lives
cmd_dir_agent:=agent/cmd/agent
cmd_dir_web:=web
cmd_dir_amberctl:=amberctl
cmd_dir_balancer:=balancer/cmd/balancer
cmd_dir_balagent:=balancer/cmd/balagent

build/bin/%: $(call image_stamp,build) docker/build-wrapper.sh $(common_go_srcs)
	$(get_vendor_submodules)
	rm -f $@
	$(call run_build_container,build,-e GOPATH=/build,$(cmd_dir_$(*F)),go install .)

.PHONY: $(foreach i,$(IMAGES) common,test-$(i))
$(foreach i,$(IMAGES) common,test-$(i)): test-%: $(call image_stamp,build)
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go test ./$*/...)

.PHONY: test
test:: $(call image_stamp,build)
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,go test $$(go list ./... | grep -v /vendor/))

.PHONY: cover
cover: $(call image_stamp,build)
	rm -rf cover
	$(get_vendor_submodules)
	$(call run_build_container,build,-e GOPATH=/build,,\
	    for d in $$(find * -path vendor -prune -o -name "*_test.go" -printf "%h\n" | sort -u); do \
	        mkdir -p cover/$$d && \
	        go test -coverprofile=cover/$$d.out $(BASEPKG)/$$d && \
	        go tool cover -html=cover/$$d.out -o cover/$$d.html ; \
	    done)

# Subdir-specific rules

include ./*/local.mk
