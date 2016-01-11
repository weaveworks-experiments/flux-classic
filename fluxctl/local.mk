# For building on host machine; assumes we're in gopath in the right
# place
bin/fluxctl: $(fluxctl_go_srcs)
	$(get_vendor_submodules)
	GO15VENDOREXPERIMENT=1 go build -o $@ ./fluxctl
