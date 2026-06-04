"""Test the actual external URL the user is using.

The user reported the issue when accessing 183.205.129.40:16000.
This is the agent-tool-host public endpoint.

This script will:
1. Open the URL in headless chromium
2. Listen to all network requests and responses
3. Listen to console logs
4. Wait for the iframe to load
5. Check what the iframe is actually requesting
6. Take screenshots
"""
from playwright.sync_api import sync_playwright
import json
import sys

URL = "http://183.205.129.40:16000/tabs/remote"

def main():
    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-setuid-sandbox"]
        )
        context = browser.new_context(
            ignore_https_errors=True,
        )
        page = context.new_page()

        # Capture all requests/responses
        requests_log = []
        console_logs = []
        page_errors = []

        def on_request(req):
            try:
                requests_log.append({
                    "type": "request",
                    "method": req.method,
                    "url": req.url,
                    "headers": dict(req.headers) if req.headers else {},
                    "resource_type": req.resource_type,
                    "post_data": req.post_data[:200] if req.post_data else None,
                })
            except Exception as e:
                requests_log.append({"type": "request_error", "error": str(e), "url": req.url})

        def on_response(resp):
            try:
                requests_log.append({
                    "type": "response",
                    "method": resp.request.method,
                    "url": resp.url,
                    "status": resp.status,
                    "headers": dict(resp.headers) if resp.headers else {},
                })
            except Exception as e:
                requests_log.append({"type": "response_error", "error": str(e), "url": resp.url})

        def on_console(msg):
            console_logs.append({
                "type": msg.type,
                "text": msg.text,
                "location": msg.location,
            })

        def on_page_error(err):
            page_errors.append(str(err))

        page.on("request", on_request)
        page.on("response", on_response)
        page.on("console", on_console)
        page.on("pageerror", on_page_error)

        print(f"Navigating to {URL} ...")
        try:
            page.goto(URL, timeout=30000, wait_until="domcontentloaded")
        except Exception as e:
            print(f"Navigation error: {e}")

        # Wait a bit for the page to render
        page.wait_for_timeout(5000)

        # Take initial screenshot
        page.screenshot(path="/tmp/initial.png", full_page=True)
        print(f"Initial screenshot saved: /tmp/initial.png")

        # Get page title
        try:
            print(f"Page title: {page.title()}")
        except Exception as e:
            print(f"Get title error: {e}")

        # Look for iframes
        frames = page.frames
        print(f"\nFound {len(frames)} frames (including main):")
        for i, frame in enumerate(frames):
            print(f"  Frame[{i}]: url={frame.url}")

        # Try to find the openlist iframe
        openlist_frame = None
        for frame in frames:
            if "openlist" in frame.url.lower():
                openlist_frame = frame
                break

        if openlist_frame:
            print(f"\n=== OpenList frame: {openlist_frame.url} ===")
            try:
                openlist_frame.wait_for_load_state("domcontentloaded", timeout=10000)
            except Exception as e:
                print(f"Frame load error: {e}")

            page.wait_for_timeout(5000)
            page.screenshot(path="/tmp/with_iframe.png", full_page=True)

            try:
                body_text = openlist_frame.locator("body").inner_text(timeout=5000)
                print(f"Iframe body text (first 1000 chars):\n{body_text[:1000]}")
            except Exception as e:
                print(f"Get iframe body text error: {e}")
        else:
            print("\nNo openlist iframe found!")

        # Print all API requests
        print(f"\n=== API requests ({len(requests_log)} total) ===")
        for entry in requests_log:
            if entry.get("type") == "request" and "/api/" in entry.get("url", ""):
                print(f"  REQ {entry.get('method')} {entry.get('url')}")
                if entry.get('headers'):
                    h = entry['headers']
                    interesting = {k: v for k, v in h.items() if k.lower() in
                                   ['origin', 'content-type', 'authorization', 'x-forwarded-prefix']}
                    if interesting:
                        print(f"       headers: {interesting}")
            elif entry.get("type") == "response" and "/api/" in entry.get("url", ""):
                print(f"  RESP {entry.get('status')} {entry.get('url')}")
                if entry.get('headers'):
                    h = entry['headers']
                    interesting = {k: v for k, v in h.items() if k.lower().startswith('access-control')}
                    if interesting:
                        print(f"        ACAO: {interesting}")

        # Print all console logs
        print(f"\n=== Console logs ({len(console_logs)} total) ===")
        for log in console_logs[-30:]:
            print(f"  [{log['type']}] {log['text']}")

        # Print page errors
        print(f"\n=== Page errors ({len(page_errors)} total) ===")
        for err in page_errors:
            print(f"  {err}")

        # Save full log
        with open("/tmp/test_log.json", "w") as f:
            json.dump({
                "requests": requests_log,
                "console": console_logs,
                "page_errors": page_errors,
            }, f, indent=2, default=str)
        print(f"\nFull log saved to /tmp/test_log.json")

        browser.close()

if __name__ == "__main__":
    main()
