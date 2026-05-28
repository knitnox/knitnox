import asyncio
import json
import os
import mimetypes
from urllib.parse import unquote
from mcp.server.fastmcp import FastMCP
from mcp.server.fastmcp.server import TransportSecuritySettings
from starlette.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse
import uvicorn
import signal
import functools
from mcp.types import PromptMessage, TextContent

# --- Configuration ---
TOOL_TIMEOUT = 30.0  # seconds - max time any single tool call can take
REQUEST_TIMEOUT = 35.0  # seconds - overall HTTP request timeout (slightly > TOOL_TIMEOUT)

# --- Blacklist patterns for tree/files resources ---
BLACKLIST_NAMES = {
    '.git', 'node_modules', '.env', 'dist', '.svelte-kit', 'build',
    '__pycache__', '.DS_Store', '.vscode', '.idea', '.next', '.nuxt',
    '.output', '.cache', 'coverage', '.nyc_output'
}
BLACKLIST_EXTENSIONS = {'.pyc', '.pyo', '.pyd', '.egg-info'}
BLACKLIST_PREFIXES = {'.env'}


def get_size_str(size_bytes: int) -> str:
    """Return human-readable size string."""
    if size_bytes < 1024:
        return f"{size_bytes} B"
    elif size_bytes < 1024 * 1024:
        return f"{size_bytes / 1024:.1f} KB"
    elif size_bytes < 1024 * 1024 * 1024:
        return f"{size_bytes / (1024 * 1024):.1f} MB"
    else:
        return f"{size_bytes / (1024 * 1024 * 1024):.2f} GB"


def is_blacklisted(name: str) -> bool:
    """Check if a file/directory name should be excluded."""
    if name in BLACKLIST_NAMES:
        return True
    if name.startswith('.'):
        return True  # Skip hidden files/folders
    if any(name.startswith(prefix) for prefix in BLACKLIST_PREFIXES):
        return True
    _, ext = os.path.splitext(name)
    if ext.lower() in BLACKLIST_EXTENSIONS:
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


# --- Timeout Middleware ---
class TimeoutMiddleware(BaseHTTPMiddleware):
    """Enforces an overall request timeout on all endpoints."""
    async def dispatch(self, request, call_next):
        try:
            return await asyncio.wait_for(call_next(request), timeout=REQUEST_TIMEOUT)
        except asyncio.TimeoutError:
            return JSONResponse(
                {"jsonrpc": "2.0", "error": {"code": -32000, "message": f"Request timed out after {REQUEST_TIMEOUT}s"}, "id": None},
                status_code=504
            )

# --- Timeout decorator for MCP tools ---
def with_timeout(timeout: float = TOOL_TIMEOUT):
    """Decorator that wraps an async tool function, enforcing an absolute timeout.
    Returns a clear MCP error instead of hanging indefinitely."""
    def decorator(func):
        @functools.wraps(func)
        async def wrapper(*args, **kwargs):
            try:
                if asyncio.iscoroutinefunction(func):
                    result = await asyncio.wait_for(func(*args, **kwargs), timeout=timeout)
                else:
                    # Run sync functions in a thread with a timeout
                    result = await asyncio.wait_for(
                        asyncio.to_thread(func, *args, **kwargs),
                        timeout=timeout
                    )
                return result
            except asyncio.TimeoutError:
                return f"Error: Tool '{func.__name__}' timed out after {timeout}s. Please try again."
            except Exception as e:
                return f"Error in tool '{func.__name__}': {str(e)}"
        return wrapper
    return decorator

# --- FastMCP Server ---
mcp = FastMCP(
    "MyTestServer",
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
    all_files = list_all_files('.')
    for f in all_files:
        path = f['path']
        mime = f['mimeType']
        
        # Create a closure to capture path and mime
        def make_reader(p, m):
            @mcp.resource(f"project://file/{p}", name=p, mime_type=m)
            def read_file_dynamic() -> str:
                # Re-read from disk to ensure freshness
                full_path = os.path.join('.', p)
                try:
                    with open(full_path, 'r', encoding='utf-8') as f_in:
                        return f_in.read()
                except Exception as e:
                    return f"Error reading {p}: {str(e)}"
            return read_file_dynamic
        
        make_reader(path, mime)

# Call registration
register_file_resources()


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

# --- Main ---

if __name__ == "__main__":
    print("Starting Streamable HTTP MCP Server on http://localhost:8000/mcp ...")
    print(f"Tool timeout: {TOOL_TIMEOUT}s | Request timeout: {REQUEST_TIMEOUT}s")

    app = mcp.streamable_http_app()

    # 1. Add timeout middleware (outermost, runs first)
    app.add_middleware(TimeoutMiddleware)

    # 2. Add CORS middleware
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
        expose_headers=["mcp-session-id"],
    )

    # Configure Uvicorn with robust settings to prevent hanging
    uvicorn.run(
        app,
        host="127.0.0.1",
        port=8000,
        timeout_keep_alive=30,          # Longer keep-alive for MCP sessions
        timeout_graceful_shutdown=10,   # Allow in-flight requests to finish
        limit_concurrency=50,           # Prevent overload from too many concurrent requests
        limit_max_requests=1000,        # Auto-restart worker after 1000 requests (prevents memory leaks)
        log_level="info",
        backlog=128,                    # Connection queue size
    )