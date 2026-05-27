from mcp.server.fastmcp import FastMCP
from mcp.server.fastmcp.server import TransportSecuritySettings
from starlette.middleware.cors import CORSMiddleware
import uvicorn

mcp = FastMCP(
    "MyTestServer", 
    host="127.0.0.1", 
    port=8000,
    transport_security=TransportSecuritySettings(
        enable_dns_rebinding_protection=False,
        allowed_origins=["*"]
    )
)

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
