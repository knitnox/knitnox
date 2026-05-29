import os
import sys
import json
from pathlib import Path

# Add current directory to sys.path to allow imports when run as a script
current_dir = str(Path(__file__).parent.resolve())
if current_dir not in sys.path:
    sys.path.append(current_dir)

import config

def setup_interactive():
    print("--- Code Server Setup ---")
    print(f"Working Directory: {os.getcwd()}")
    
    config_dict = {
        "server": {
            "name": "LocalAssistantServer",
            "host": "127.0.0.1",
            "port": 8000
        },
        "memory": {
            "enabled": False
        }
    }
    
    use_memory = input("Do you want to use memory-related tools (Knowledge Graph)? (y/n): ").lower().strip() == 'y'
    
    if use_memory:
        config_dict["memory"]["enabled"] = True
        print("\nConfiguring Memory Tools:")
        print("You need an OpenAI-compatible embedding API.")
        print("Suggestions:")
        print("  - Ollama: http://localhost:11434/v1 (Model: mxbai-embed-large)")
        print("  - OpenRouter: https://openrouter.ai/api/v1 (Model: qwen/qwen3-embedding-8b)")
        
        base_url = input("Embedding API Base URL [https://openrouter.ai/api/v1]: ").strip()
        if not base_url:
            base_url = "https://openrouter.ai/api/v1"
            
        model = input("Embedding Model [qwen/qwen3-embedding-8b]: ").strip()
        if not model:
            model = "qwen/qwen3-embedding-8b"
            
        api_key = input("API Key (leave blank for local like Ollama): ").strip()
        
        config_dict["memory"]["embedding"] = {
            "base_url": base_url,
            "model": model,
            "api_key": api_key
        }
        config_dict["memory"]["db_path"] = "mcp_memory.db"
    
    config.save_config(config_dict)
    print(f"\nConfiguration saved to {config.CONFIG_PATH}")

def run():
    # 1. Initialize config for current directory
    config.init_config(os.getcwd())
    
    # 2. If no config exists, run setup
    if not os.path.exists(config.CONFIG_PATH) or not config._config:
        setup_interactive()
    
    # 3. Run the server
    import main
    main.run_server()

if __name__ == "__main__":
    run()
