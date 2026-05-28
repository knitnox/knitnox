# knitnox

**knitnox** is a modern, high-performance AI chat interface built with SvelteKit. It is designed to be a unified hub for AI interactions, featuring deep integration with the **Model Context Protocol (MCP)**, allowing it to autonomously discover and utilize external tools, resources, and prompts.

## 🚀 Key Features

-   **Generic MCP Resource Viewer:** A structural-aware inspector that autonomously detects and renders trees, lists, and raw content from any MCP server.
-   **Intelligent Prompt Routing:** Support for complex, multi-turn MCP prompts that can override system instructions and pre-populate chat history.
-   **Granular Context Management:** Attach individual files or structured data to your chat context with one click.
-   **Advanced Agent Loop:** Autonomous tool-use loop with configurable max turns and context window management.
-   **Modern UI/UX:** Built with Tailwind CSS and Lucide icons, featuring real-time toast notifications, markdown rendering with code highlighting, and auto-scrolling.

## 🧠 Software Philosophy & Architecture

### Philosophy: "Generic Interoperability"
knitnox follows a "Generic First" philosophy. Instead of building specialized UI for specific tools (like a "File Explorer"), we build generic data viewers that adapt to the shape of the data provided by the Model Context Protocol. This ensures that as the MCP ecosystem grows, knitnox remains compatible with any server without code changes.

### High-Level Architecture
-   **Frontend:** SvelteKit (Svelte 5) for a reactive and efficient UI.
-   **State Management:** Svelte 5 "Runes" (`$state`, `$derived`, `$effect`) for fine-grained reactivity.
-   **Storage:** Dexie.js (IndexedDB) for local-first chat history and metadata storage.
-   **AI Integration:** Unified streaming interface for OpenAI-compatible APIs (OpenAI, Ollama, Groq, etc.).
-   **MCP Client:** A custom, robust MCP implementation supporting Tool calling, Resource reading, and Prompt rendering.

## 📁 Component Tree

```text
src/
├── lib/
│   ├── components/
│   │   ├── Chat.svelte             # Main conversation engine & message list
│   │   ├── Sidebar.svelte          # Chat history & navigation
│   │   ├── MCPInspectModal.svelte  # The generic resource & prompt browser
│   │   ├── SettingsModal.svelte    # Global configuration (API keys, Models)
│   │   ├── ToolInspectModal.svelte # Deep dive into tool calls/results
│   │   ├── Toaster.svelte          # Notification system
│   │   └── LandingPage.svelte      # Zero-state welcome screen
│   ├── mcp.svelte.ts               # MCP Client Pool & Tool discovery logic
│   ├── resource-context.svelte.ts  # Shared state for pending chat attachments
│   ├── prompt-context.svelte.ts    # Logic for routing complex MCP prompts
│   └── toast.svelte.ts             # Global notification store
└── routes/
    └── +page.svelte                # Root application layout
```

## 🛠 Developer Guide

### Prerequisites
-   Node.js (v18+)
-   Python 3.10+ (for running the test MCP server)

### Setup
1.  **Install dependencies:**
    ```bash
    npm install
    ```
2.  **Run the development server:**
    ```bash
    npm run dev
    ```
3.  **Launch the Test MCP Server:**
    ```bash
    python test-mcp-server.py
    ```

### Working with MCP
knitnox uses a dynamic registration system. To add a new server, simply go to **Settings** and add the server URL (e.g., `http://localhost:8000/mcp`).

#### Generic Viewers
The `MCPInspectModal` uses content-based detection. If you return JSON from a resource, the UI will attempt to parse it:
-   If it has `children: []`, it renders a **Tree**.
-   If it's an array of objects `[]`, it renders a **List**.
-   Otherwise, it renders **Raw Text**.

#### Prompt Workaround
Because some MCP SDKs strictly limit roles to `user` and `assistant`, knitnox supports a "JSON-in-User" workaround. If a prompt returns a single user message containing a JSON array of messages, knitnox will parse it and:
1.  Override the **System Prompt** if a system role is found.
2.  Populate **Chat History** with intermediate turns.
3.  Place the final message in the **User Input Box**.

## 📄 License
MIT
