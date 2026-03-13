/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package main

import (
	"flag"
	"fmt"
	"os"

	"go.arpabet.com/glue/internal/gluegen"
)

func main() {
	var check bool
	flag.BoolVar(&check, "check", false, "fail if generated files are stale")
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	result, err := gluegen.Generate(gluegen.Options{
		Dir:      ".",
		Patterns: patterns,
		Write:    !check,
		Check:    check,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, path := range result.Generated {
		fmt.Println(path)
	}
	for _, path := range result.Removed {
		fmt.Println(path)
	}
}
