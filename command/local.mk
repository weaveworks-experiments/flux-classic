COMMAND_SRC:= $(shell find command -name '*.go')

command.bin: $(COMMAND_SRC)

# For building on host machine; assumes we're in gopath in the right
# place
amberctl: $(COMMAND_SRC)
	go get ./command
	go build -o $@ ./command
