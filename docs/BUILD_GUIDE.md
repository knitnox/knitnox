# Build guide

How to turn the source code into a program you can run, on Windows, Linux and macOS, and how to build for a different system than the one you are on.

> **Good to know.** Knitnox builds into **one file**. Pages, styles, scripts, fonts, guides and database migrations are all inside it. There is no C code, so you only need Go.

## What you need

| Tool | Version | Why |
|---|---|---|
| **Go** | 1.26 or newer | Compiles the program |
| **Git** | any | Only to download the source |

You do **not** need Node, a C compiler, Docker or a database server.

Check your Go version:

```bash
go version
```

If the command is not found, or the version is older than 1.26, install Go from `https://go.dev/dl/` and open a new terminal.

## Build on Windows

Open **PowerShell** in the project folder.

```powershell
go build -trimpath -ldflags="-s -w" -o knitnox.exe ./cmd/knitnox
```

Run it:

```powershell
.\knitnox.exe
```

Then open `http://localhost:8080`. The first account you create is the admin.

To use another port:

```powershell
$env:KNITNOX_ADDR = ":3000"; .\knitnox.exe
```

## Build on Linux

Open a terminal in the project folder.

```bash
go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
```

Run it:

```bash
./knitnox
```

Then open `http://localhost:8080`.

To use another port:

```bash
KNITNOX_ADDR=:3000 ./knitnox
```

## Build on macOS

Open **Terminal** in the project folder. The command is the same as on Linux and works on both Apple Silicon and Intel Macs.

```bash
go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
./knitnox
```

A program you built yourself runs without warnings. A build that was **downloaded** on a Mac is blocked by Gatekeeper the first time. Clear the block once:

```bash
xattr -d com.apple.quarantine ./knitnox
```

## What the build flags mean

| Flag | What it does | Needed? |
|---|---|---|
| `-o knitnox` | Names the output file | Yes |
| `-trimpath` | Removes your folder names from the file | Recommended |
| `-ldflags="-s -w"` | Removes debug information: about 15 MB instead of 25 MB | Recommended |
| `./cmd/knitnox` | The program to build | Yes |

While developing, skip the build and run directly:

```bash
go run ./cmd/knitnox
```

## Cross-compilation: build for another system

You can build for any system from any system. Set two variables before the build:

- **`GOOS`**: the target operating system (`windows`, `linux`, `darwin` for macOS).
- **`GOARCH`**: the target processor (`amd64` for Intel/AMD, `arm64` for ARM).

### Which target do I need?

| The program will run on… | `GOOS` | `GOARCH` | Output name |
|---|---|---|---|
| Most Linux servers, VPS, Docker | `linux` | `amd64` | `knitnox` |
| Raspberry Pi 4/5, AWS Graviton, Oracle free tier | `linux` | `arm64` | `knitnox` |
| Mac with Apple Silicon (M1 and later) | `darwin` | `arm64` | `knitnox` |
| Mac with Intel | `darwin` | `amd64` | `knitnox` |
| Most Windows PCs | `windows` | `amd64` | `knitnox.exe` |
| Windows on ARM (Snapdragon laptops) | `windows` | `arm64` | `knitnox.exe` |

Not sure about a Linux machine? Run `uname -m` on it: `x86_64` means `amd64`, `aarch64` means `arm64`.

### From Linux or macOS (Bash, zsh)

Put the variables in front of the command. They apply to that one command only.

```bash
# for a Linux server
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox

# for a Raspberry Pi
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox

# for Windows
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o knitnox.exe ./cmd/knitnox

# for a Mac with Apple Silicon
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
```

### From Windows (PowerShell)

Set the variables, build, then **remove them**. If you forget, every later build still targets the other system.

```powershell
# for a Linux server
$env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
Remove-Item Env:GOOS, Env:GOARCH
```

```powershell
# for a Mac with Apple Silicon
$env:GOOS = "darwin"; $env:GOARCH = "arm64"
go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
Remove-Item Env:GOOS, Env:GOARCH
```

### From Windows (Git Bash)

Git Bash accepts the same form as Linux and macOS:

```bash
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o knitnox ./cmd/knitnox
```

## Build every target at once

One script builds all six targets into the `dist` folder, packs each one, and writes a checksum file.

| You are on | Command | Packages |
|---|---|---|
| Linux, macOS, Git Bash | `scripts/build-all.sh 1.0.0` | `.tar.gz` for Linux and macOS, `.zip` for Windows |
| Windows PowerShell | `.\scripts\build-all.ps1 -Version 1.0.0` | `.zip` for every target |

The files are named `knitnox-1.0.0-<os>-<arch>`, and `dist/SHA256SUMS.txt` lists their checksums. Leave the version out to get `dev`.

## Move the build to another machine

1. Copy the file to the target machine.
2. On Linux and macOS, make it runnable:

   ```bash
   chmod +x knitnox
   ```

3. Start it:

   ```bash
   ./knitnox                 # Linux, macOS
   .\knitnox.exe             # Windows
   ```

4. Open `http://<that-machine>:8080` and create the first account.

Data is stored in a `data` folder next to where you start it. Set `KNITNOX_DATA` to choose another place.

## Build with Docker

```bash
docker build -t knitnox .
docker run -p 8080:8080 -v knitnox-data:/data -e KNITNOX_SECRET=change-me knitnox
```

For an ARM server, build the ARM image from any machine:

```bash
docker build --platform linux/arm64 -t knitnox .
```

## Check before you ship

```bash
gofmt -l .        # should print nothing
go vet ./...      # should print nothing
go test ./...     # every package should say "ok"
```

`go test -race ./...` needs a C compiler. It is the one command here that does; run it on Linux in CI if you want it.

## When the build fails

| What you see | What to do |
|---|---|
| `go: command not found` | Install Go, then open a new terminal |
| `go.mod requires go >= 1.26` | Your Go is too old. Install a newer one |
| `pattern ...: no matching files found` | A file that is built into the program is missing. Check that `docs/` and `internal/web/` are complete |
| `exec format error` when starting | The file was built for another system or processor. Rebuild with the right `GOOS` and `GOARCH` |
| "cannot execute binary file" on Linux | Same cause as above |
| `permission denied` on Linux or macOS | Run `chmod +x knitnox` |
| Later builds on Windows produce a Linux file | `GOOS` is still set. Run `Remove-Item Env:GOOS, Env:GOARCH` |
| "address already in use" when starting | Another program uses port 8080. Set `KNITNOX_ADDR` to another port |
| Page changes do not show up | Pages and styles are inside the program. Build or `go run` again |

## Where to go next

The README covers running Knitnox as a service, the reverse proxy, backups and upgrades.
