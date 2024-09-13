package main

import (
	"fmt"
	"github.com/ndeta100/gomon/handlers"
	"os"
)

func main() {
	// Retrieve command-line arguments
	args := os.Args[1:]

	// Define a map of commands to handler functions
	var commandHandlers = map[string]func([]string){
		"init":  handlers.InitHandler,
		"watch": handlers.WatchHandler,
	}

	// Check if at least one command-line argument is provided
	if len(args) == 0 {
		fmt.Println("No command provided. Available commands: init, watch")
		os.Exit(1)
	}

	// Find and execute the command handler
	command := args[0]
	if handler, exists := commandHandlers[command]; exists {
		handler(args[1:]) // Pass remaining arguments to the handler
	} else {
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}
