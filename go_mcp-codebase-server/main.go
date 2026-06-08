package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	port := flag.Int("port", 8080, "Server port")
	rootDir := flag.String("root", ".", "Root directory for file operations")
	allowedDirs := flag.String("allow", "", "Comma-separated list of additional allowed directories")
	flag.Parse()

	// Resolve root directory to absolute path
	absRoot, err := getAbsPath(*rootDir)
	if err != nil {
		log.Fatalf("Invalid root directory: %v", err)
	}

	// Parse allowed directories
	extraDirs := []string{}
	if *allowedDirs != "" {
		for _, d := range strings.Split(*allowedDirs, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				abs, err := getAbsPath(d)
				if err == nil {
					extraDirs = append(extraDirs, abs)
				}
			}
		}
	}

	// Create MCP server with tools
	mcpServer := newMCPServer(absRoot, extraDirs)

	// Create both transport handlers from the same MCP server
	streamableHandler := newStreamableHTTPHandler(mcpServer)
	sseHandler := newSSEHandler(mcpServer)

	// Set up HTTP multiplexer to support both transports
	mux := http.NewServeMux()
	mux.Handle("/mcp", streamableHandler)        // Streamable HTTP transport
	mux.Handle("/sse", sseHandler)               // SSE transport (event stream)
	mux.Handle("/message", sseHandler)            // SSE transport (message posting)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Default: return info page
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "MCP Codebase Server\n\nEndpoints:\n  Streamable HTTP: POST /mcp\n  SSE: GET /sse + POST /messages\n")
	})

	// Wrap with CORS middleware
	handler := corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	// Detect LAN IP for network access URL
	lanIP := ""
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				lanIP = ipNet.IP.String()
				break
			}
		}
	}

	go func() {
		log.Printf("──────────────────────────────────────────────")
		log.Printf("🚀 MCP Codebase Server starting on %s", addr)
		log.Printf("📁 Root directory: %s", absRoot)
		if len(extraDirs) > 0 {
			log.Printf("📂 Additional allowed dirs: %v", extraDirs)
		}
		log.Printf("🌐 CORS: allowing all origins")
		log.Printf("🔗 Streamable HTTP: http://localhost:%d/mcp", *port)
		log.Printf("🔗 SSE transport:   http://localhost:%d/sse", *port)
		if lanIP != "" {
			log.Printf("🔗 Network HTTP:    http://%s:%d/mcp", lanIP, *port)
			log.Printf("🔗 Network SSE:     http://%s:%d/sse", lanIP, *port)
		}
		log.Printf("──────────────────────────────────────────────")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("Server stopped")
}