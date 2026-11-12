# Install guide

How to download Knitnox and start it on Windows, macOS or Linux. You do not need Go, Docker or a database. Knitnox is **one file** that you unpack and run.

> **Good to know.** Windows and macOS show a warning the first time you open Knitnox. That is expected, and "Why your computer shows a warning" below explains it. Linux shows no warning.

## Pick your download

All downloads are on the [releases page](https://github.com/knitnox/knitnox/releases/latest). These links always point to the newest version:

| Your computer | File |
|---|---|
| Windows PC (most of them) | [knitnox-windows-amd64.zip](https://github.com/knitnox/knitnox/releases/latest/download/knitnox-windows-amd64.zip) |
| Windows on ARM (Snapdragon laptops) | [knitnox-windows-arm64.zip](https://github.com/knitnox/knitnox/releases/latest/download/knitnox-windows-arm64.zip) |
| Mac with Apple Silicon (M1 and later) | [knitnox-macos-arm64.tar.gz](https://github.com/knitnox/knitnox/releases/latest/download/knitnox-macos-arm64.tar.gz) |
| Mac with Intel | [knitnox-macos-amd64.tar.gz](https://github.com/knitnox/knitnox/releases/latest/download/knitnox-macos-amd64.tar.gz) |
| Linux PC or server (Intel/AMD) | [knitnox-linux-amd64.tar.gz](https://github.com/knitnox/knitnox/releases/latest/download/knitnox-linux-amd64.tar.gz) |
| Raspberry Pi 4/5 (64-bit), ARM servers | [knitnox-linux-arm64.tar.gz](https://github.com/knitnox/knitnox/releases/latest/download/knitnox-linux-arm64.tar.gz) |

Not sure which Mac you have? Open the Apple menu → **About This Mac**. "Chip: Apple M…" means Apple Silicon; "Processor: Intel" means Intel. On Linux, run `uname -m`: `x86_64` means amd64, `aarch64` means arm64.

## Install on Windows

1. Download the `.zip` and unpack it: right-click → **Extract All**.
2. Open the new folder and double-click **knitnox.exe**.
3. A blue box says **"Windows protected your PC"**. Click **More info**, then **Run anyway**. You only do this once.
4. If Windows Firewall asks, click **Allow** when phones or other computers on your network should reach Knitnox. Click **Cancel** to use it on this PC only.
5. A black window opens and stays open. That window *is* Knitnox; closing it stops Knitnox.
6. Open `http://localhost:8080` in your browser.

Your browser may also say the file "isn't commonly downloaded". Choose **Keep** (in Edge: **⋯ → Keep → Show more → Keep anyway**).

To skip the blue box instead: right-click `knitnox.exe` → **Properties** → tick **Unblock** → **OK**.

## Install on macOS

The simplest way is the Terminal (Applications → Utilities → Terminal). A file fetched this way is not marked as "from the internet", so macOS normally shows no warning.

```bash
# Apple Silicon (M1 and later). For an Intel Mac, write amd64 in place of arm64.
curl -LO https://github.com/knitnox/knitnox/releases/latest/download/knitnox-macos-arm64.tar.gz
tar -xzf knitnox-macos-arm64.tar.gz
cd knitnox-macos-arm64
./knitnox
```

Then open `http://localhost:8080`.

**If you downloaded the file with your browser**, macOS says *"knitnox" Not Opened: Apple could not verify…* Do one of these, once:

- Click **Done** on that message, open **System Settings → Privacy & Security**, scroll down to **Security**, click **Open Anyway** next to "knitnox", and confirm with your password. Then start it again.
- Or remove the mark in the Terminal, inside the unpacked folder:

  ```bash
  xattr -d com.apple.quarantine knitnox
  ```

On macOS 14 and older you can also right-click the file and choose **Open**.

## Install on Linux

```bash
# Intel/AMD. For a Raspberry Pi or ARM server, write arm64 in place of amd64.
curl -LO https://github.com/knitnox/knitnox/releases/latest/download/knitnox-linux-amd64.tar.gz
tar -xzf knitnox-linux-amd64.tar.gz
cd knitnox-linux-amd64
./knitnox
```

Then open `http://localhost:8080`, or `http://<the-server's-address>:8080` from another computer. To keep it running after you log out and start it at boot, see "Deployment" in the README (systemd).

## First start

1. Open `http://localhost:8080` and **create an account. The first account becomes the admin.**
2. Connect an AI under **Admin → Providers → Add provider**:
   - **OpenAI:** base URL `https://api.openai.com/v1` and your API key.
   - **Free and fully local with [Ollama](https://ollama.com):** base URL `http://localhost:11434/v1`, any key.
   - Any other OpenAI-compatible service works the same way.
3. Click **Fetch models from provider**, tick the models people may use, and start a chat.

Knitnox keeps everything in a `data` folder next to where you started it: the database, your uploads and daily backups. **Keep that folder; it is all your data.**

To use another port, set `KNITNOX_ADDR` before starting: `KNITNOX_ADDR=:3000 ./knitnox` (PowerShell: `$env:KNITNOX_ADDR=":3000"; .\knitnox.exe`).

## Why your computer shows a warning

The warning does not mean something was found in the file. It means the file carries **no paid signature**.

Windows and macOS trust a program without questions only when its maker has paid for a certificate that puts a verified name on it:

| What | Who is paid | Price |
|---|---|---|
| Signing and notarising for macOS | Apple Developer Program | US$99 every year |
| Code-signing certificate for Windows | A certificate company | about US$200–400 every year |
| **Together** | | **US$300 or more, every year** |

Microsoft has a cheaper signing service (about US$10 a month), but it is not open to individual developers in every country. And even a newly signed Windows program keeps showing the warning until enough people have downloaded it.

Knitnox is a free hobby project. It earns nothing, so we chose not to pay several hundred dollars a year to remove a warning. That money buys a name on a certificate; it does not make the program safer.

What you get instead:

- **The source is public.** Anyone can read every line at `github.com/knitnox/knitnox`.
- **Downloads are built by GitHub**, straight from that public source, by a workflow you can read (`.github/workflows/release.yml`). Nobody builds them on a private computer.
- **Every release lists checksums**, so you can check that your download is exactly the file that was built.
- **You can build it yourself** with one command. See the [Build guide](/help/build).

Only download Knitnox from `github.com/knitnox/knitnox`. A copy from anywhere else has none of these guarantees.

## Check your download

Each release has a `SHA256SUMS.txt`. Compare its line for your file with what your computer calculates:

```bash
sha256sum knitnox-linux-amd64.tar.gz          # Linux
shasum -a 256 knitnox-macos-arm64.tar.gz      # macOS
```

```powershell
Get-FileHash knitnox-windows-amd64.zip        # Windows PowerShell
```

The long string must be the same as in `SHA256SUMS.txt` (capital or small letters do not matter).

## Update to a new version

1. Back up: **Admin → Settings → Backups → Back up now**.
2. Stop Knitnox (close its window, or press `Ctrl+C`).
3. Download the new version and replace the old `knitnox` (or `knitnox.exe`) file with the new one. **Leave the `data` folder where it is.**
4. Start it again. The database is brought up to date by itself.

`knitnox version` prints the version you have. On Windows and on a Mac where you used the browser, the warning appears once more for the new file.

## When something goes wrong

| What you see | What to do |
|---|---|
| "Windows protected your PC" | **More info → Run anyway** |
| *"knitnox" Not Opened* on a Mac | **System Settings → Privacy & Security → Open Anyway** |
| `permission denied` on Linux or macOS | Run `chmod +x knitnox` |
| `exec format error`, "cannot execute binary file", "Bad CPU type" | The download is for another processor. Take the other file (amd64 ↔ arm64) |
| "address already in use" | Another program uses port 8080. Set `KNITNOX_ADDR` to another port |
| The page does not open from another device | Allow Knitnox in the firewall, and use the computer's network address in place of `localhost` |
| The black window closes right away on Windows | Open PowerShell in the folder and run `.\knitnox.exe` to read the message |
