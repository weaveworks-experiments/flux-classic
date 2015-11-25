PROJ:=ambergreen
BASEPKG:=github.com/squaremo/$(PROJ)
IMAGES=balancer agent web amberctl
BUILD_IMAGES=build webbuild

image_stamp=docker/.$1.done

.PHONY: images
images: $(foreach i,$(IMAGES),docker/$(i).tar)

.PHONY: clean
clean::
	rm -rf build cover
	rm -f $(foreach i,$(IMAGES),docker/.$(i).done) docker/*.tar

.PHONY: realclean
realclean:: clean
	rm -rf $(foreach i,build webbuild,$(call image_stamp,$(i)))

# Don't remove this if a subsequent step fails
.PRECIOUS: $(call image_stamp,build)

$(foreach i,$(IMAGES) $(BUILD_IMAGES),docker/.$(i).done): docker/.%.done: docker/Dockerfile.%
	rm -rf build-container
	mkdir build-container
	cp -pr $^ build-container/
	docker build -t $(PROJ)/$(*F) -f build-container/$(<F) build-container
	rm -rf build-container
	touch $@

$(foreach i,$(IMAGES),docker/$(i).tar): docker/%.tar: docker/.%.done
	docker save --output=$@ $(PROJ)/$(*F)

$(foreach i,$(IMAGES),$(eval $(call image_stamp,$(i)): build/bin/$(i)))
$(foreach i,$(IMAGES) common,$(eval $(i)_go_srcs:=$(shell find $(i) -name '*.go')))
$(foreach i,$(IMAGES),$(eval build/bin/$(i): $($(i)_go_srcs)))

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

build/bin/%: $(call image_stamp,build) docker/build-wrapper.sh $(common_go_srcs)
	$(get_vendor_submodules)
	rm -f $@
	$(call run_build_container,build,-e GOPATH=/build,$(*F),go install ./...)

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
