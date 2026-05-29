# Code-Server: Local MCP for Assistant

`code-server` is a portable, globally installable Model Context Protocol (MCP) server that provides your AI assistants (like Claude Desktop, Gemini CLI, etc.) with surgical file manipulation, web scraping, and long-term Knowledge Graph memory.

Unlike static servers, `code-server` is context-aware. Run it in any directory, and it instantly treats that folder as its workspace, maintaining isolated configurations and memories for every project.

---

## 🚀 Quick Start

### Installation
From the root of this repository, install it globally using `pip` or `uv`:

```bash
# Using uv (recommended)
uv tool install .

# Or using pip
pip install -e .
```

### Uninstallation
To remove the global command:

```bash
# Using uv
uv tool uninstall code-server

# Using pip
pip uninstall code-server
```

### Usage
Simply navigate to any project folder and run:
```bash
code-server
```

If it's your first time running it in that folder, the CLI will guide you through an interactive setup to configure your memory (Knowledge Graph) preferences.

### Development Mode
To run the server during development without installing it globally:

```bash
# Using uv
uv run cli.py

# Using python directly
python cli.py
```

---

## 🛠 Features

- **📂 Project-Local Context:** Automatically creates a `.code-server/` folder in your current directory to store `config.json` and `mcp_memory.db`.
- **🧠 Knowledge Graph Memory:** Stores facts and relationships in a Kuzu graph database with vector embeddings for semantic search.
- **✂️ Surgical File Edits:** Tools to read, create, and apply precise patches to files without rewriting the entire file.
- **🌐 Web Intelligence:** Scrape static or JS-heavy websites and perform web searches on the fly.
- **💻 Terminal Access:** Safely execute shell commands (with configurable forbidden commands).

---

## ⚠️ Troubleshooting

### Global Command Ignoring Code Changes (The UV Cache Gotcha)
If you are developing or applying fixes to the code, you might notice that the global `code-server` command completely ignores your changes, even if you run an uninstall/reinstall.

This happens because `uv tool install` creates a **static wheel cache** based on the version number in `pyproject.toml`. If you don't change the version number, `uv` will just reinstall the old cached version!

To properly force an update when modifying the source code:
1. Open `pyproject.toml` and **bump the `version`** (e.g., from `0.1.0` to `0.1.1`).
2. Run the forced install command:
```bash
# The version bump is REQUIRED to break the uv cache
uv tool install --force .
```

### Immediate Shutdown on Start
If the server starts and immediately shuts down without a clear error:
1. Ensure no other instance is running (which might lock the Kuzu database).
2. Delete the `.code-server` directory in your current folder and run `code-server` again to trigger a fresh setup.

---

## 📁 Project Structure

```text
Local MCP for Assistant/
├── cli.py             # CLI Entry point & interactive setup logic
├── main.py            # FastMCP server definition & tool registration
├── config.py          # Dynamic configuration manager (handles .code-server/)
├── memory.py          # Knowledge Graph & Vector Search (Kuzu + OpenAI/Ollama)
├── file_patcher.py    # Surgical file operations & diff logic
├── scraping.py        # Web scraping (BeautifulSoup + Playwright)
├── system.py          # Terminal command execution & security filters
└── pyproject.toml     # Project metadata & global script definition
```

---

## 📘 User Guide

### Configuration
Each project folder contains its own configuration in `.code-server/config.json`. You can manually edit this to:
- Change the server **Port** or **Host**.
- Update **Embedding API** details (e.g., switching from OpenRouter to Ollama).
- Modify the **Blacklist** to hide specific files/folders from the AI.

### Memory Tools (Optional)
If enabled, the server provides:
- `save_memory`: Extract facts from conversation and store them in the graph.
- `search_memory`: Semantic search for past context.
- `explore_graph`: Visualize connections between entities.

### File Tools
- `read_file`: Reads file with line numbers.
- `apply_patch`: The most powerful tool. It uses `difflib` to find the best match for `old_text` and replace it with `new_text`.

---

## 👩‍💻 Developer Guide

### Dynamic Config Pattern
The project uses a "Base Directory" pattern. When `cli.py` starts, it calls `config.init_config(os.getcwd())`. This re-bases all internal paths (database, scripts, config) to the current working directory.

### Adding New Tools
1. Create a new module (or use an existing one like `system.py`).
2. Define your `async` or `sync` function.
3. In `main.py`, import the module and register it using the `@mcp.tool()` decorator.
   - *Note:* If the tool requires specific configuration, ensure it accesses variables from `config.py` *dynamically* (inside the function) to support runtime config changes.

### Environment
- **Python:** 3.12+
- **Key Libraries:** `mcp` (FastMCP), `kuzu` (Graph DB), `uvicorn` (Server), `playwright` (Scraping).

---

## 🔒 Security
- **Forbidden Commands:** `rm`, `sudo`, `shutdown`, etc., are blocked by default in `system.py`.
- **Path Traversal:** File tools are restricted to the current working directory.
- **CORS:** Configurable via `config.json`.
