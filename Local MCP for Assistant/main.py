import asyncio
import json
import os
import mimetypes
from urllib.parse import unquote

# Ensure config is initialized based on the current working directory,
# especially if main.py is run directly instead of through cli.py
import config
config.init_config(os.getcwd())

from mcp.server.fastmcp import FastMCP
from mcp.server.fastmcp.server import TransportSecuritySettings
from starlette.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse
import uvicorn
import signal
import functools
from mcp.types import PromptMessage, TextContent

# Import tools from modules
import file_patcher
import memory
import scraping
import system
import config

def get_size_str(size_bytes: int) -> str:
    """Return human-readable size string."""
    if size_bytes < 1024:
        return f"{size_bytes} B"
    elif size_bytes < 1024 * 1024:
        return f"{size_bytes / 1024:.1f} KB"
    elif size_bytes < 1024 * 1024 * 1024:
        return f"{size_bytes / (1024 * 1024):.1f} MB"
    else:
        return f"{size_bytes / (1024 * 1024 * 1024):.20f} GB"


def is_blacklisted(name: str) -> bool:
    """Check if a file/directory name should be excluded."""
    if name in config.BLACKLIST_NAMES:
        return True
    if name.startswith('.'):
        return True  # Skip hidden files/folders
    if any(name.startswith(prefix) for prefix in config.BLACKLIST_PREFIXES):
        return True
    _, ext = os.path.splitext(name)
    if ext.lower() in config.BLACKLIST_EXTENSIONS:
        return True
    return False


def guess_mime(path: str) -> str:
    """Guess MIME type from file extension."""
    mime, _ = mimetypes.guess_type(path)
    return mime or 'application/octet-stream'


def build_file_tree(root_path: str = '.') -> dict:
    """Build a recursive file/folder tree as a dict, excluding blacklisted items."""
    name = os.path.basename(root_path) or root_path
    node = {
        'name': name,
        'type': 'directory',
        'size': 0,
        'children': []
    }
    try:
        entries = sorted(os.listdir(root_path), key=lambda x: (not os.path.isdir(os.path.join(root_path, x)), x.lower()))
    except (PermissionError, OSError):
        return node

    for entry in entries:
        full_path = os.path.join(root_path, entry)
        if is_blacklisted(entry):
            continue
        try:
            if os.path.isdir(full_path):
                child = build_file_tree(full_path)
                node['children'].append(child)
            else:
                stat = os.stat(full_path)
                content = ""
                try:
                    with open(full_path, 'r', encoding='utf-8') as f:
                        content = f.read()
                except Exception:
                    content = "[Binary or unreadable content]"
                
                node['children'].append({
                    'name': entry,
                    'type': 'file',
                    'size': stat.st_size,
                    'sizeStr': get_size_str(stat.st_size),
                    'mimeType': guess_mime(full_path),
                    'content': content
                })
        except (PermissionError, OSError):
            continue

    # Compute directory size as sum of children (recursive)
    total_size = 0
    for child in node['children']:
        if child['type'] == 'file':
            total_size += child.get('size', 0)
        else:
            total_size += child.get('size', 0)  # already computed recursively
    node['size'] = total_size
    node['sizeStr'] = get_size_str(total_size)
    return node


def list_all_files(root_path: str = '.') -> list:
    """Return a flat list of all files (not dirs) with metadata and content."""
    files = []
    for dirpath, dirnames, filenames in os.walk(root_path):
        # Filter dirnames in-place to skip blacklisted dirs
        dirnames[:] = [d for d in dirnames if not is_blacklisted(d)]
        for fname in filenames:
            if is_blacklisted(fname):
                continue
            full_path = os.path.join(dirpath, fname)
            rel_path = os.path.relpath(full_path, root_path).replace('\\', '/')
            try:
                stat = os.stat(full_path)
                content = ""
                try:
                    with open(full_path, 'r', encoding='utf-8') as f:
                        content = f.read()
                except Exception:
                    content = "[Binary or unreadable content]"

                files.append({
                    'name': fname,
                    'path': rel_path,
                    'size': stat.st_size,
                    'sizeStr': get_size_str(stat.st_size),
                    'mimeType': guess_mime(full_path),
                    'content': content
                })
            except (PermissionError, OSError):
                continue
    return sorted(files, key=lambda x: x['path'])


# --- Timeout decorator for MCP tools ---
def with_timeout(timeout: float = None):
    """Decorator that wraps an async tool function, enforcing an absolute timeout.
    Returns a clear MCP error instead of hanging indefinitely."""
    def decorator(func):
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            # Resolve timeout at call time
            actual_timeout = timeout if timeout is not None else config.TOOL_TIMEOUT
            try:
                if asyncio.iscoroutinefunction(func):
                    result = await asyncio.wait_for(func(*args, **kwargs), timeout=actual_timeout)
                else:
                    # Run sync functions in a thread with a timeout
                    result = await asyncio.wait_for(
                        asyncio.to_thread(func, *args, **kwargs),
                        timeout=actual_timeout
                    )
                return result
            except asyncio.TimeoutError:
                return f"Error: Tool '{func.__name__}' timed out after {actual_timeout}s. Please try again."
            except Exception as e:
                return f"Error in tool '{func.__name__}': {str(e)}"
        return wrapper
    return decorator

# --- FastMCP Server ---
mcp = FastMCP(
    "LocalAssistantServer",
    host="127.0.0.1",
    port=8000,
    transport_security=TransportSecuritySettings(
        enable_dns_rebinding_protection=False,
        allowed_origins=["*"]
    )
)

# --- Resources ---

@mcp.resource("system://users")
def list_users() -> str:
    """Returns a generic JSON array of user objects (testing list viewer)."""
    users = [
        {"id": 1, "name": "Alice", "role": "Admin", "email": "alice@example.com"},
        {"id": 2, "name": "Bob", "role": "User", "email": "bob@example.com"},
        {"id": 3, "name": "Charlie", "role": "Guest", "email": "charlie@example.com"}
    ]
    return json.dumps(users, indent=2)


@mcp.resource("system://org-chart")
def org_chart() -> str:
    """Returns a generic JSON tree (testing tree viewer)."""
    chart = {
        "name": "Acme Corp",
        "type": "organization",
        "children": [
            {
                "name": "Engineering",
                "type": "department",
                "children": [
                    {"name": "Frontend Team", "type": "team", "children": []},
                    {"name": "Backend Team", "type": "team", "children": []}
                ]
            },
            {
                "name": "Product",
                "type": "department",
                "children": [
                    {"name": "Design", "type": "team", "children": []}
                ]
            }
        ]
    }
    return json.dumps(chart, indent=2)


@mcp.resource("system://logs")
def system_logs() -> str:
    """Returns raw text logs."""
    return "[INFO] 2026-05-28 10:00:00 - Server started\n[DEBUG] 2026-05-28 10:00:01 - Initializing plugins\n[ERROR] 2026-05-28 10:05:22 - Failed to connect to database"


def register_file_resources():
    """Dynamically register every file in the project as an individual resource."""
    # Use current directory as root
    root = "."
    all_files = list_all_files(root)
    for f in all_files:
        path = f['path']
        mime = f['mimeType']
        
        # Create a closure to capture path and mime
        def make_reader(p, m):
            @mcp.resource(f"project://file/{p}", name=p, mime_type=m)
            def read_file_dynamic() -> str:
                # Re-read from disk to ensure freshness
                full_path = os.path.join(root, p)
                try:
                    with open(full_path, 'r', encoding='utf-8') as f_in:
                        return f_in.read()
                except Exception as e:
                    return f"Error reading {p}: {str(e)}"
            return read_file_dynamic
        
        make_reader(path, mime)


@mcp.resource("project://file/{path}")
def read_file_resource(path: str) -> str:
    """Reads a specific file from disk. Returns file content with metadata wrapper."""
    try:
        # Decode URL-encoded path segments (like %2F for /)
        path = unquote(path)
        # Resolve relative to current dir, prevent path traversal
        safe_path = os.path.normpath(path)

        if os.path.isabs(safe_path) or safe_path.startswith('..'):
            return json.dumps({"error": f"Invalid path: {path}. Only relative paths within the project are allowed."})

        full_path = os.path.join('.', safe_path)
        full_path = os.path.normpath(full_path)

        if not os.path.exists(full_path):
            return json.dumps({"error": f"File not found: {path}"})
        if not os.path.isfile(full_path):
            return json.dumps({"error": f"Not a file: {path}"})

        stat = os.stat(full_path)
        with open(full_path, 'r', encoding='utf-8') as f:
            content = f.read()

        result = {
            'name': os.path.basename(full_path),
            'path': path,
            'size': stat.st_size,
            'sizeStr': get_size_str(stat.st_size),
            'mimeType': guess_mime(full_path),
            'content': content
        }
        return json.dumps(result, indent=2)
    except UnicodeDecodeError:
        return json.dumps({"error": f"Cannot read binary file as text: {path}"})
    except Exception as e:
        return json.dumps({"error": f"Error reading file: {str(e)}"})


# --- Prompts ---

@mcp.prompt("analyze-project")
def analyze_project():
    """A prompt to analyze the current project structure."""
    return "Please look at the files in this directory and explain the project's purpose and architecture."

@mcp.prompt("debug-assistant")
def debug_assistant(error_msg: str = "Unknown error"):
    """A prompt to help debug a specific error."""
    return f"I am encountering the following error: '{error_msg}'. Can you help me find the root cause in my code?"

@mcp.prompt("coding-assistant")
def coding_assistant():
    """A complex structured prompt testing system override and history.
    Returns a JSON string of messages within a single user message to bypass SDK role restrictions.
    """
    messages = [
        {
            "role": "system",
            "content": "You are a senior full-stack engineer. Your tone is professional, concise, and focused on clean code."
        },
        {
            "role": "user",
            "content": "Hello! I need help refactoring my project."
        },
        {
            "role": "assistant",
            "content": "I'd be happy to help. What specific part would you like to start with?"
        },
        {
            "role": "user",
            "content": "Can you check the current directory and suggest improvements for the API layer?"
        }
    ]
    return json.dumps(messages)

@mcp.prompt("test-list")
def test_list():
    """Simple list-based prompt for testing."""
    return [
        PromptMessage(role="user", content=TextContent(type="text", text="Message 1")),
        PromptMessage(role="assistant", content=TextContent(type="text", text="Response 1")),
        PromptMessage(role="user", content=TextContent(type="text", text="Message 2"))
    ]

# --- Tools ---

@mcp.tool()
def add_numbers(a: int, b: int) -> int:
    """Adds two numbers together."""
    return a + b

@mcp.tool()
def echo(message: str) -> str:
    """Returns the message back to the user. Runs in < 1s guaranteed."""
    return f"Echo: {message}"

@mcp.tool()
def health() -> str:
    """Health check tool. Returns 'ok' if the server is responsive.
    Use this as a pre-flight check before making other tool calls."""
    return "ok"

# --- Tools from Modules ---

@mcp.tool()
@with_timeout(60.0)
async def read_file(filepath: str) -> str:
    """
    Reads a file and returns its content with line numbers.
    
    Use this to examine the code, configuration, or content of a specific file. 
    The path should be relative to the current directory.
    
    Parameters:
    - filepath: The name or relative path of the file to read.
    """
    return await file_patcher.read_file(filepath)

@mcp.tool()
@with_timeout(60.0)
async def apply_patch(filepath: str, old_text: str = "", new_text: str = "", action: str = "edit") -> str:
    """
    Create, edit, or delete files using precise text matching.
    
    This tool allows for surgical edits to files. 
    - For 'edit': Matches 'old_text' to find the exact location for replacement with 'new_text'. 
      Always provide a unique and sufficient context in 'old_text' to avoid ambiguity.
    - For 'create': Creates a new file with 'new_text' as its content.
    - For 'delete': Removes the specified file.
    
    Parameters:
    - filepath: The name or relative path of the file to modify.
    - old_text: The exact text segment to be replaced (required for 'edit').
    - new_text: The new text to insert for 'edit' or the full content for 'create'.
    - action: The action to perform: 'create', 'edit', or 'delete'.
    """
    return await file_patcher.apply_patch(filepath, old_text, new_text, action)

@mcp.tool()
async def list_files_in_scripts(directory: str = "") -> str:
    """
    Lists all files and directories within the current folder.
    
    Use this to discover available files or understand the structure of the 
    directory before reading or modifying files.
    
    Parameters:
    - directory: An optional subdirectory to list.
    """
    return await file_patcher.list_files(directory)

if config.MEMORY_ENABLED:
    @mcp.tool()
    @with_timeout(60.0)
    async def save_memory(user_id: str, text: str, relations: list) -> str:
        """
        Saves a memory to the graph database and builds connections in the Knowledge Graph.
        
        This tool stores natural language text as a memory and extracts structured 
        knowledge (entities and relationships) to build a dynamic Knowledge Graph.

        Parameters:
        - user_id: The unique identifier for the user (e.g., "user123").
        - text: The original natural language text stated by the user.
        - relations: A list of dictionaries representing the facts.
          Each dictionary MUST have:
          - "head": Human-readable name of the source entity (e.g., "Apple").
          - "head_id": UNIQUE identifier for the source entity (e.g., "apple_company").
          - "relation": Relationship type in SCREAMING_SNAKE_CASE (e.g., "WORKS_AT").
          - "tail": Human-readable name of the target entity (e.g., "iPhone").
          - "tail_id": UNIQUE identifier for the target entity (e.g., "iphone_product").
        """
        return await memory.save_memory(user_id, text, relations)

    @mcp.tool()
    @with_timeout(60.0)
    async def search_memory(query: str, k: int = 5) -> str:
        """
        Search for relevant memories using semantic (vector) similarity.
        
        Use this tool when you need to recall past interactions, preferences, or facts 
        mentioned by the user that might not be directly linked in the knowledge graph 
        but are semantically related to the query.

        Parameters:
        - query: The natural language search query or topic to find memories about.
        - k: The maximum number of relevant memories to return (default is 5).
        """
        return await memory.search_memory(query, k)

    @mcp.tool()
    @with_timeout(60.0)
    async def explore_graph(entity_id: str) -> str:
        """
        Explore the knowledge graph by traversing relationships connected to a specific entity ID.
        
        This tool is essential for finding deeper context and understanding the web of 
        relationships surrounding an entity. It returns all outgoing and incoming 
        relationships for the given entity ID.

        Parameters:
        - entity_id: The UNIQUE identifier for the entity to explore (e.g., "apple_company").
        """
        return await memory.explore_graph(entity_id)

    @mcp.tool()
    @with_timeout(60.0)
    async def explore_graph_deep(entity_id: str) -> str:
        """
        Perform a deep (2-hop) exploration of the knowledge graph starting from an entity ID.
        
        Use this when you need a broader context or want to discover indirect connections 
        (e.g., "A knows B, and B knows C"). It retrieves direct neighbors AND their connections.
        
        Parameters:
        - entity_id: The UNIQUE identifier for the starting entity (e.g., "apple_company").
        """
        return await memory.explore_graph_deep(entity_id)

@mcp.tool()
@with_timeout(60.0)
async def scrape_url(url: str) -> str:
    """
    Scrapes the content from a specified URL and returns clean text.
    
    Best for static websites without heavy JavaScript. It removes HTML tags, 
    scripts, and styles to provide readable text content.
    
    Parameters:
    - url: The full URL of the website to scrape.
    """
    return await scraping.scrape(url)

@mcp.tool()
@with_timeout(120.0)
async def scrape_js_url(url: str, wait_time: int = 3) -> str:
    """
    Scrapes JavaScript-rendered content from a URL using a headless browser.
    
    Essential for modern web apps (React, Vue, etc.) where content is loaded dynamically.
    The 'wait_time' allows the page to finish rendering before extraction.
    
    Parameters:
    - url: The full URL of the website to scrape.
    - wait_time: Seconds to wait after page load (default 3).
    """
    return await scraping.scrape_js(url, wait_time)

@mcp.tool()
@with_timeout(60.0)
async def search_web(query: str, max_results: int = 5) -> str:
    """
    Performs a web search using DuckDuckGo and returns summarized results.
    
    Use this to find up-to-date information, documentation, or answers to 
    questions that require external knowledge.
    
    Parameters:
    - query: The search query.
    - max_results: Maximum number of results to return (default 5).
    """
    return await scraping.search_web(query, max_results)

@mcp.tool()
@with_timeout(60.0)
async def run_terminal_command(command: str) -> str:
    """
    Executes a shell command asynchronously and returns the output.
    
    Use this for system-level operations like checking process status, 
    running scripts, or other CLI-based tasks. Includes security filters for 
    dangerous commands.
    
    Parameters:
    - command: The shell command to execute.
    """
    return await system.run_terminal_command(command)



# --- Main ---

async def startup():
    """Initialize resources on startup."""
    if config.MEMORY_ENABLED:
        print("Initializing memory database and clients...")
        # Initialize Kuzu and embedding client
        await memory.init_memory_db()
        memory.init_client()
    print("Startup complete.")

def run_server():
    print(f"Starting Local MCP Server on http://{config.HOST}:{config.PORT}/mcp ...")
    print(f"Working Directory: {config.BASE_DIR}")
    print(f"Tool timeout: {config.TOOL_TIMEOUT}s | Request timeout: {config.REQUEST_TIMEOUT}s")

    # Register file resources now that config is fully initialized
    register_file_resources()

    app = mcp.streamable_http_app()

    # Add startup event handler
    app.add_event_handler("startup", startup)

    # Add CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=config.CORS_ORIGINS,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
        expose_headers=["mcp-session-id"],
    )

    # Configure Uvicorn with robust settings to prevent hanging
    uvicorn.run(
        app,
        host=config.HOST,
        port=config.PORT,
        timeout_keep_alive=30,          # Longer keep-alive for MCP sessions
        timeout_graceful_shutdown=10,   # Allow in-flight requests to finish
        limit_concurrency=50,           # Prevent overload from too many concurrent requests
        limit_max_requests=1000,        # Auto-restart worker after 1000 requests (prevents memory leaks)
        log_level="info",
        backlog=128,                    # Connection queue size
    )

if __name__ == "__main__":
    run_server()
