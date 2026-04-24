//go:build linux

/*
To build, run `go build -o oGShell-client cmd/client/main.go`
Usage: `./oGShell-client <host> <port>`
Example: `./oGShell-client 172.168.1.1 4444`
*/
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

type termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Cc     [20]byte
	Ispeed uint32
	Ospeed uint32
}

const (
	TCGETS    = 0x5401
	TCSETS    = 0x5402
	ECHO      = 0x8
	ICANON    = 0x2
	ISIG      = 0x1
	SYS_IOCTL = 16 // Linux amd64
)

func getTerminalSize() (int, int, error) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := &winsize{}
	_, _, errno := syscall.RawSyscall(SYS_IOCTL,
		os.Stdout.Fd(),
		0x5413, // TIOCGWINSZ
		uintptr(unsafe.Pointer(ws)))
	if errno != 0 {
		return 0, 0, errno
	}
	return int(ws.Row), int(ws.Col), nil
}

func getTermios(fd int) (*termios, error) {
	t := &termios{}
	_, _, errno := syscall.RawSyscall(SYS_IOCTL,
		uintptr(fd),
		TCGETS,
		uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return nil, errno
	}
	return t, nil
}

func setTermios(fd int, t *termios) error {
	_, _, errno := syscall.RawSyscall(SYS_IOCTL,
		uintptr(fd),
		TCSETS,
		uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func makeRaw(fd int) (*termios, error) {
	old, err := getTermios(fd)
	if err != nil {
		return nil, err
	}

	raw := *old
	// Disable echo, canonical mode, and signal processing
	raw.Lflag &^= ECHO | ICANON | ISIG
	// Minimum 1 byte read, no timeout
	raw.Cc[6] = 1 // VMIN
	raw.Cc[5] = 0 // VTIME

	if err := setTermios(fd, &raw); err != nil {
		return nil, err
	}
	return old, nil
}

func restoreTermios(fd int, old *termios) {
	setTermios(fd, old)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: oGShell-client <port>")
		os.Exit(1)
	}

	port := os.Args[1]

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Printf("Error listening: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()

	fmt.Printf("Listening on port %s...\n", port)

	conn, err := ln.Accept()
	if err != nil {
		fmt.Printf("Error accepting: %v\n", err)
		os.Exit(1)
	}

	oldState, err := makeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Printf("Error setting raw mode: %v\n", err)
		os.Exit(1)
	}

	cleanup := func() {
		restoreTermios(int(os.Stdin.Fd()), oldState)
		conn.Close()
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cleanup()
		fmt.Println("\r\nDisconnected.")
		os.Exit(0)
	}()

	// Send terminal size
	rows, cols, err := getTerminalSize()
	if err == nil {
		conn.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", rows, cols)))
	}

	done := make(chan struct{}, 2)

	// Remote -> local
	go func() {
		io.Copy(os.Stdout, conn)
		done <- struct{}{}
	}()

	// Local -> remote
	go func() {
		io.Copy(conn, os.Stdin)
		done <- struct{}{}
	}()

	<-done
	cleanup()
	fmt.Println("\r\nConnection closed.")
}
