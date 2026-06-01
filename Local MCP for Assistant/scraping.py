"""Web scraping tools extracted for MCP."""
import asyncio
import aiohttp
from bs4 import BeautifulSoup
from playwright.async_api import async_playwright
try:
    from ddgs import DDGS
except ImportError:
    from duckduckgo_search import DDGS
import config

# Configuration from config.py
PLAYWRIGHT_TIMEOUT = config.PLAYWRIGHT_TIMEOUT
DEFAULT_WAIT_TIME = config.DEFAULT_WAIT_TIME

async def fetch(session, url):
    async with session.get(url) as response:
        return await response.text()

async def scrape(url: str) -> str:
    """Scrapes the content from the specified URL and returns clean text without HTML tags. Please provide a valid URL. Use this for static HTML websites."""
    async with aiohttp.ClientSession() as session:
        async with session.get(url) as response:
            html = await response.text()
        soup = BeautifulSoup(html, "html.parser")

        # Remove unwanted tags
        for tag in soup(["script", "style", "a"]):
            tag.decompose()

        # Extract clean text
        text = soup.get_text(separator="\n", strip=True)
        return text

import sys
import subprocess

async def ensure_playwright_browsers():
    """Checks if playwright browsers are installed, and installs them if not."""
    try:
        # We try to launch a browser to see if it's installed. 
        # A simpler way is to check the playwright executable, but launching is more definitive.
        async with async_playwright() as p:
            try:
                browser = await p.chromium.launch(headless=True)
                await browser.close()
                return True, ""
            except Exception as e:
                if "Executable doesn't exist" in str(e) or "not installed" in str(e).lower():
                    print("Playwright Chromium not found. Attempting to install...")
                    # Run the install command
                    process = subprocess.Popen(
                        [sys.executable, "-m", "playwright", "install", "chromium"],
                        stdout=subprocess.PIPE,
                        stderr=subprocess.PIPE,
                        text=True
                    )
                    stdout, stderr = process.communicate()
                    if process.returncode == 0:
                        return True, "Chromium was successfully installed."
                    else:
                        return False, f"Failed to install Chromium: {stderr}"
                raise e
    except Exception as e:
        return False, f"Error checking/installing Playwright: {str(e)}"

async def scrape_js(url: str, wait_time: int = DEFAULT_WAIT_TIME) -> str:
    """Scrapes JavaScript-rendered content from the specified URL using a headless browser. Use this for JavaScript-heavy websites (React, Vue, Angular, SPAs). The wait_time parameter (default 3 seconds) allows the page to fully load before extracting content."""
    try:
        # Ensure browsers are installed
        installed, message = await ensure_playwright_browsers()
        status_msg = ""
        if message:
            status_msg = f"INFO: {message}\n\n"

        async with async_playwright() as p:
            # Launch headless browser
            browser = await p.chromium.launch(headless=True)
            page = await browser.new_page()
            
            # Navigate to URL and wait for network to be idle
            await page.goto(url, wait_until="networkidle", timeout=PLAYWRIGHT_TIMEOUT)
            
            # Additional wait time for dynamic content
            await page.wait_for_timeout(wait_time * 1000)
            
            # Get the rendered HTML
            html = await page.content()
            
            # Close browser
            await browser.close()
            
            # Parse with BeautifulSoup
            soup = BeautifulSoup(html, "html.parser")
            
            # Remove unwanted tags
            for tag in soup(["script", "style", "a"]):
                tag.decompose()
            
            # Extract clean text
            text = soup.get_text(separator="\n", strip=True)
            return text
            
    except Exception as e:
        error_msg = f"Error scraping JavaScript content from {url}: {str(e)}"
        return error_msg

async def search_web(query: str, max_results: int = 5) -> str:
    """Searches the web using DuckDuckGo and returns results with titles, URLs, and snippets. Use this to find information on the internet."""
    try:
        # Define the synchronous search function
        def sync_search():
            # Use DDGS() inside a context manager to ensure proper cleanup
            with DDGS() as ddgs:
                # Use positional argument for the query to handle both 'keywords' (old) 
                # and 'query' (new) parameter names across different DDGS versions.
                try:
                    results = list(ddgs.text(query, max_results=max_results))
                except Exception:
                    # Fallback to backend='lite' if default fails
                    results = list(ddgs.text(query, max_results=max_results, backend="lite"))
                return results
        
        # Offload the blocking function to a background thread
        results = await asyncio.to_thread(sync_search)
        
        if not results:
            return f"No results found for '{query}'. Try a different query."

        # Format the results
        formatted_results = []
        for i, result in enumerate(results, 1):
            title = result.get('title', 'No Title')
            href = result.get('href', 'No URL')
            body = result.get('body', 'No Snippet')
            formatted_results.append(
                f"Result {i}:\n"
                f"Title: {title}\n"
                f"URL: {href}\n"
                f"Snippet: {body}\n"
            )
        
        output = "\n".join(formatted_results)
        return output
        
    except Exception as e:
        error_msg = f"Error searching the web for '{query}': {str(e)}"
        return error_msg
