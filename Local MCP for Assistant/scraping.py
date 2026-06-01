"""Web scraping tools extracted for MCP."""
import asyncio
import aiohttp
from bs4 import BeautifulSoup
from playwright.async_api import async_playwright
from playwright_stealth import Stealth
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

async def browse_web(url: str, wait_time: int = DEFAULT_WAIT_TIME) -> str:
    """
    Browses a website using a headless browser, simulating a real user. 
    Use this for any website, especially those with dynamic content (React, SPAs). 
    Returns the clean text content and a list of useful links found on the page.
    """
    try:
        # Ensure browsers are installed
        installed, message = await ensure_playwright_browsers()
        status_msg = ""
        if message:
            status_msg = f"INFO: {message}\n\n"

        async with async_playwright() as p:
            # Launch headless browser. 
            # We remove --disable-http2 as it can be a detection signal for modern UAs.
            browser = await p.chromium.launch(
                headless=True,
                args=[
                    "--disable-blink-features=AutomationControlled",
                    "--no-sandbox",
                    "--disable-setuid-sandbox",
                    "--disable-infobars",
                    "--window-position=0,0",
                    "--ignore-certificate-errors",
                    "--ignore-certificate-errors-spki-list",
                ]
            )
            
            # Use a more comprehensive set of headers
            context = await browser.new_context(
                user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
                viewport={'width': 1920, 'height': 1080},
                java_script_enabled=True,
                accept_downloads=True,
                extra_http_headers={
                    "Accept-Language": "en-US,en;q=0.9",
                    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
                    "Upgrade-Insecure-Requests": "1",
                    "Sec-Fetch-Site": "none",
                    "Sec-Fetch-Mode": "navigate",
                    "Sec-Fetch-User": "?1",
                    "Sec-Fetch-Dest": "document",
                    "sec-ch-ua": '"Google Chrome";v="123", "Not:A-Brand";v="8", "Chromium";v="123"',
                    "sec-ch-ua-mobile": "?0",
                    "sec-ch-ua-platform": '"Windows"',
                }
            )
            page = await context.new_page()
            
            # Use playwright-stealth for robust evasion
            await Stealth().apply_stealth_async(page)
            
            # Navigate to URL
            try:
                # Use "networkidle" for Cloudflare/Akamai as they often need time to run verification scripts
                await page.goto(url, wait_until="networkidle", timeout=PLAYWRIGHT_TIMEOUT)
            except Exception as e:
                # Fallback if networkidle takes too long
                if "timeout" in str(e).lower():
                    try:
                        await page.goto(url, wait_until="load", timeout=PLAYWRIGHT_TIMEOUT)
                    except Exception:
                        pass # Continue with whatever we have
                else:
                    raise e
            
            # Additional wait time for dynamic content
            await page.wait_for_timeout(wait_time * 1000)
            
            # Get the rendered HTML
            html = await page.content()
            
            # Extract links before decomposing tags
            soup = BeautifulSoup(html, "html.parser")
            
            # Find unique links that are likely useful
            links = []
            seen_urls = {url.rstrip('/')}
            from urllib.parse import urljoin
            for a in soup.find_all('a', href=True):
                original_href = a['href']
                # Resolve relative URLs
                href = urljoin(url, original_href)
                
                clean_url = href.split('#')[0].rstrip('/')
                if clean_url.startswith('http') and clean_url not in seen_urls:
                    link_text = a.get_text(strip=True)
                    if link_text and len(link_text) > 2:
                        links.append(f"{link_text}: {clean_url}")
                        seen_urls.add(clean_url)
                
                if len(links) >= 15: # Limit links to avoid overwhelming context
                    break

            # Remove unwanted tags for text extraction
            for tag in soup(["script", "style", "nav", "footer", "header"]):
                tag.decompose()
            
            # Extract clean text
            text = soup.get_text(separator="\n", strip=True)
            
            # Close browser
            await browser.close()
            
            # Format output
            output = f"{status_msg}--- CONTENT FROM {url} ---\n\n"
            output += text[:10000] # Limit text length
            if len(text) > 10000:
                output += "\n\n[Content truncated due to length...]"
                
            if links:
                output += "\n\n--- USEFUL LINKS FOUND ---\n"
                output += "\n".join(links)
                
            return output
            
    except Exception as e:
        error_msg = f"Error browsing {url}: {str(e)}"
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
