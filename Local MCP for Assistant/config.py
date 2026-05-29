import json
import os
from pathlib import Path

# Defaults
BASE_DIR = Path(os.getcwd())
CONFIG_DIR = BASE_DIR / ".code-server"
CONFIG_PATH = CONFIG_DIR / "config.json"

_config = {}

def init_config(base_path=None):
    global BASE_DIR, CONFIG_DIR, CONFIG_PATH, _config
    if base_path:
        BASE_DIR = Path(base_path).resolve()
    else:
        BASE_DIR = Path(os.getcwd()).resolve()
        
    CONFIG_DIR = BASE_DIR / ".code-server"
    CONFIG_PATH = CONFIG_DIR / "config.json"
    
    if not CONFIG_DIR.exists():
        CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        
    if CONFIG_PATH.exists():
        with open(CONFIG_PATH, "r") as f:
            _config = json.load(f)
    else:
        _config = {}
    
    # Refresh globals
    global SERVER_NAME, HOST, PORT, TOOL_TIMEOUT, REQUEST_TIMEOUT, CORS_ORIGINS
    global KUZU_DB_PATH, EMBEDDING_MODEL, EMBEDDING_API_KEY, EMBEDDING_BASE_URL
    global SCRIPTS_DIR, PLAYWRIGHT_TIMEOUT, DEFAULT_WAIT_TIME
    global FORBIDDEN_COMMANDS, COMMAND_TIMEOUT, BLACKLIST_NAMES, BLACKLIST_EXTENSIONS, BLACKLIST_PREFIXES
    global MEMORY_ENABLED

    SERVER_NAME = get("server.name", "LocalAssistantServer")
    HOST = get("server.host", "127.0.0.1")
    PORT = get("server.port", 8000)
    TOOL_TIMEOUT = get("server.tool_timeout", 30.0)
    REQUEST_TIMEOUT = get("server.request_timeout", 35.0)
    CORS_ORIGINS = get("server.cors_origins", ["*"])

    MEMORY_ENABLED = get("memory.enabled", True)
    KUZU_DB_PATH = str(CONFIG_DIR / get("memory.db_path", "mcp_memory.db"))
    
    # Embedding settings: config.json takes absolute precedence. 
    # Environment variables are only used if the key is completely missing from config.json.
    _model = get("memory.embedding.model")
    EMBEDDING_MODEL = _model if _model is not None else (os.environ.get("EMBEDDING_MODEL") or "qwen/qwen3-embedding-8b")
    
    _api_key = get("memory.embedding.api_key")
    EMBEDDING_API_KEY = _api_key if _api_key is not None else (os.environ.get("EMBEDDING_API_KEY") or os.environ.get("OPENAI_API_KEY") or "")
    
    _base_url = get("memory.embedding.base_url")
    EMBEDDING_BASE_URL = _base_url if _base_url is not None else (os.environ.get("EMBEDDING_BASE_URL") or "https://openrouter.ai/api/v1")

    SCRIPTS_DIR = str(BASE_DIR)

    PLAYWRIGHT_TIMEOUT = get("scraping.playwright_timeout", 90000)
    DEFAULT_WAIT_TIME = get("scraping.default_wait_time", 3)

    FORBIDDEN_COMMANDS = get("system.forbidden_commands", ["rm ", "sudo ", "shutdown ", "reboot "])
    COMMAND_TIMEOUT = get("system.command_timeout", 20)

    BLACKLIST_NAMES = set(get("blacklist.names", []))
    BLACKLIST_EXTENSIONS = set(get("blacklist.extensions", []))
    BLACKLIST_PREFIXES = set(get("blacklist.prefixes", []))

def get(key_path, default=None):
    """Get a value from the config using a dot-separated path."""
    keys = key_path.split(".")
    val = _config
    for key in keys:
        if isinstance(val, dict) and key in val:
            val = val[key]
        else:
            return default
    return val

def save_config(config_dict):
    global _config
    _config = config_dict
    if not CONFIG_DIR.exists():
        CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    with open(CONFIG_PATH, "w") as f:
        json.dump(config_dict, f, indent=4)
    init_config(BASE_DIR)

# Initialize with defaults on import
init_config()
