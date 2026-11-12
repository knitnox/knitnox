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

Closing the PowerShell window stops Knitnox, and nothing keeps what it printed. To keep a log, set a log file before starting:

```powershell
$env:KNITNOX_LOG_FILE = "knitnox.log"; .\knitnox.exe
```

To have it start with Windows and come back after a failure, add it to **Task Scheduler** ("At startup", and under Settings "If the task fails, restart every 1 minute").

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

The files are named `knitnox-<os>-<arch>` (`macos` stands for Go's `darwin`), and `dist/SHA256SUMS.txt` lists their checksums. The names carry no version on purpose, so a download link never changes. The version is built into the program: `knitnox version` prints it. Leave the version out to get `dev`.

The `dist` folder is not kept in Git. Public downloads come from GitHub Releases, described next.

## Release a new version

Downloads for users are made by GitHub, not on your computer. Pushing a version tag starts `.github/workflows/release.yml`. It runs the checks, builds all six targets with `scripts/build-all.sh`, and publishes a **GitHub Release** with the six archives and `SHA256SUMS.txt`.

1. Make sure `main` is what you want to ship and its **test** run on GitHub is green.
2. Pick the version number: `v` plus three numbers.

   | Change | Example |
   |---|---|
   | Fixes only | `v1.0.0` → `v1.0.1` |
   | New features, nothing breaks | `v1.0.1` → `v1.1.0` |
   | Admins must do something by hand to upgrade | `v1.1.0` → `v2.0.0` |

3. Tag and push:

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

4. Watch the **release** run under the repository's **Actions** tab. It takes a few minutes.
5. Open **Releases**, read the notes GitHub wrote from the commits, and edit them into a few plain sentences: what is new, and anything an admin must know before upgrading.

That is all. The website and the [Install guide](/help/install) link to `https://github.com/knitnox/knitnox/releases/latest/download/<file>`, which always serves the newest release, so they need no change.

Good to know:

- **Try it first.** A tag with a hyphen, like `v1.1.0-rc1`, becomes a *pre-release*. It gets its own page and downloads, but "latest" keeps pointing at the last real release.
- **A release went wrong?** Delete the release and its tag on GitHub (`gh release delete v1.0.0 --cleanup-tag`), fix the problem, and tag again. If people may already have downloaded it, publish a new number instead.
- **Do not rename the files.** The website, the Install guide and the README link to them by name. If you add a target, add it to both build scripts and to those three places.
- **Downloads are not signed**, so Windows and macOS warn on first start. The Install guide explains this to users, with the prices: US$99 a year for Apple, about US$200–400 a year for a Windows certificate. To start signing one day, add the certificate as a repository secret and a signing step after the build in `release.yml`; nothing else changes.
- **First release only:** if the run fails with a permission error, check under **Settings → Actions → General → Workflow permissions** that the organisation lets workflows write. The workflow asks for `contents: write` itself, which is normally enough.

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

Data is stored in a `data` folder next to where you start it. Set `KNITNOX_DATA` to choose another place. Automatic backups go to `data/backups`; set `KNITNOX_BACKUP_DIR` to a folder on another disk.

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

`go test -race ./...` needs a C compiler. It is the one command here that does. The file `.github/workflows/test.yml` runs it on Linux for every push.

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

The README covers running Knitnox as a service, the reverse proxy, backups, forgotten passwords and upgrades.
