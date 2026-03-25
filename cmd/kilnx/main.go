package main

import (
	"fmt"
	"os"

	"github.com/kilnx-org/kilnx/internal/runtime"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		if len(os.Args) < 3 {
			fmt.Println("Usage: kilnx run <file.kilnx>")
			os.Exit(1)
		}
		if err := runtime.WatchAndServe(os.Args[2], 8080); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("kilnx v0.1.0")
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: kilnx <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run <file.kilnx>    Start the server")
	fmt.Println("  version             Print version")
}
