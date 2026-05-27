from mcp.server.fastmcp import FastMCP
from mcp.server.fastmcp.server import TransportSecuritySettings
from starlette.middleware.cors import CORSMiddleware
import uvicorn
import os

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
        # Security: prevent directory traversal
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
    """Returns the message back to the user."""
    return f"Echo: {message}"

if __name__ == "__main__":
    print("Starting Streamable HTTP MCP Server on http://localhost:8000/mcp ...")
    app = mcp.streamable_http_app()
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
        expose_headers=["mcp-session-id"],
    )
    uvicorn.run(app, host="127.0.0.1", port=8000)
