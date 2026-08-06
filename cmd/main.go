package main

import (
	"fmt"

	"kbase/internal/version"
)

func main() {
	fmt.Printf("kbase %s — hello, world\n", version.Short())
}
