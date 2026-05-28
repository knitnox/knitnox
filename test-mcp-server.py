import asyncio
from mcp.server.fastmcp import FastMCP
from mcp.server.fastmcp.server import TransportSecuritySettings
from starlette.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse
import uvicorn
import os
import signal
import functools

# --- Configuration ---
TOOL_TIMEOUT = 30.0  # seconds - max time any single tool call can take
REQUEST_TIMEOUT = 35.0  # seconds - overall HTTP request timeout (slightly > TOOL_TIMEOUT)

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

@mcp.resource("file://list")
def list_current_dir() -> str:
    """Lists files and folders in the current directory."""
    files = os.listdir('.')
    return "\n".join([f"[DIR] {f}" if os.path.isdir(f) else f"[FILE] {f}" for f in files])

@mcp.resource("file://{path}")
def read_file_resource(path: str) -> str:
    """Reads a specific file from the disk."""
    try:
        safe_path = os.path.basename(path)
        if not os.path.isfile(safe_path):
            return f"Error: File '{safe_path}' not found."
        with open(safe_path, 'r', encoding='utf-8') as f:
            return f.read()
    except Exception as e:
        return f"Error reading file: {str(e)}"

# --- Prompts ---

@mcp.prompt("analyze-project")
def analyze_project():
    """A prompt to analyze the current project structure."""
    return "Please look at the files in this directory and explain the project's purpose and architecture."

@mcp.prompt("debug-assistant")
def debug_assistant(error_msg: str):
    """A prompt to help debug a specific error."""
    return f"I am encountering the following error: '{error_msg}'. Can you help me find the root cause in my code?"

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