"""Web scraping tools extracted for MCP."""
import asyncio
import aiohttp
from bs4 import BeautifulSoup
from playwright.async_api import async_playwright
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

async def scrape_js(url: str, wait_time: int = DEFAULT_WAIT_TIME) -> str:
    """Scrapes JavaScript-rendered content from the specified URL using a headless browser. Use this for JavaScript-heavy websites (React, Vue, Angular, SPAs). The wait_time parameter (default 3 seconds) allows the page to fully load before extracting content."""
    try:
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
                # Wrap in list() to resolve the generator before returning
                return list(ddgs.text(query, max_results=max_results))
        
        # Offload the blocking function to a background thread
        results = await asyncio.to_thread(sync_search)
        
        # Format the results
        formatted_results = []
        for i, result in enumerate(results, 1):
            formatted_results.append(
                f"Result {i}:\n"
                f"Title: {result['title']}\n"
                f"URL: {result['href']}\n"
                f"Snippet: {result['body']}\n"
            )
        
        output = "\n".join(formatted_results)
        return output
        
    except Exception as e:
        error_msg = f"Error searching the web for '{query}': {str(e)}"
        return error_msg
