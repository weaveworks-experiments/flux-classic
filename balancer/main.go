package main

import (
	"fmt"
	"os"

	"github.com/squaremo/ambergreen/balancer/interceptor"
)

func main() {
	err := interceptor.Main()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
