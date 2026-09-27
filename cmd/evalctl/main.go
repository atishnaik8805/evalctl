// Command evalctl is a single-binary, language-agnostic evaluation
// framework CLI (contracts/cli-commands.md).
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "run":
		err = runCommand(os.Args[2:])
	case "calibrate":
		err = calibrateCommand(os.Args[2:])
	case "validate":
		err = validateCommand(os.Args[2:])
	case "list-graders":
		err = listGradersCommand(os.Args[2:])
	case "analyze":
		err = analyzeCommand(os.Args[2:])
	case "-h", "--help", "help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "evalctl: unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "evalctl: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `evalctl - universal evaluation framework

Usage:
  evalctl run --suite <path> [--output <path>]
  evalctl calibrate --human <path> --model <path>
  evalctl validate --suite <path>
  evalctl list-graders
  evalctl analyze --traces <path> [--annotations <path>] [--addr host:port] [--llm-endpoint <url>] [--llm-api-key-env <VAR>]`)
}
