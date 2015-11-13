COMMAND_SRC:= $(shell find amberctl -name '*.go')

amberctl.bin: $(COMMAND_SRC)

# For building on host machine; assumes we're in gopath in the right
# place
bin/amberctl: $(COMMAND_SRC)
	$(get_vendor_submodules)
	GO15VENDOREXPERIMENT=1 go build -o $@ ./amberctl
