//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/merill/msgraph/internal/apidocs"
)

func main() {
	idx, err := apidocs.LoadIndex(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := apidocs.BuildFTSDatabase(idx, os.Args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Built FTS DB: %d endpoints, %d resources\n",
		idx.EndpointCount, idx.ResourceCount)
}
