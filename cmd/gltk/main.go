// Command gltk is the GrokLangToolKit CLI (GLVM bytecode virtual machine).
package main

import (
	"os"

	"groklang/gltk/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}
