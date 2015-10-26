package main

import (
	"fmt"
	"os"

	"github.com/dpw/ambergris/interceptor"
)

func main() {
	err := interceptor.Main()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
