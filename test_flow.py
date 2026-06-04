"""Simulate the user's actual flow:
1. Open /tabs/openlist
2. Wait for LocalOpenListStatusCard to show 'running'
3. Click "Open Web UI" button (openWebUi)
4. Should navigate to OpenListWebView which contains iframe /openlist-spa/#/login
5. Verify iframe loads and axios calls work
"""
from playwright.sync_api import sync_playwright
import json

URL = "http://127.0.0.1:16000/tabs/openlist"

def main():
    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-setuid-sandbox"]
        )
        context = browser.new_context(ignore_https_errors=True)
        page = context.new_page()

        requests_log = []
        console_logs = []
        page_errors = []

        page.on("request", lambda req: requests_log.append({
            "type": "request", "method": req.method, "url": req.url,
            "headers": dict(req.headers) if req.headers else {},
        }))
        page.on("response", lambda resp: requests_log.append({
            "type": "response", "method": resp.request.method, "url": resp.url,
            "status": resp.status, "headers": dict(resp.headers) if resp.headers else {},
        }))
        page.on("console", lambda msg: console_logs.append({"type": msg.type, "text": msg.text}))
        page.on("pageerror", lambda err: page_errors.append(str(err)))

        print(f"Navigating to {URL} ...")
        try:
            page.goto(URL, timeout=30000, wait_until="domcontentloaded")
        except Exception as e:
            print(f"Navigation error: {e}")

        page.wait_for_timeout(8000)
        page.screenshot(path="/tmp/flow_1_remote.png", full_page=True)

        # Look for the "Open Web UI" button
        try:
            # The button text might be in Chinese - "打开 Web UI"
            btns = page.locator("ion-button").all()
            print(f"\n=== {len(btns)} ion-button elements ===")
            for i, b in enumerate(btns):
                txt = b.text_content() or ""
                print(f"  btn[{i}]: {txt[:80]}")
        except Exception as e:
            print(f"Get buttons error: {e}")

        # Try to find open-webui-btn class
        try:
            open_btn = page.locator(".open-webui-btn").first
            if open_btn.is_visible():
                print(f"\nClicking open-webui-btn...")
                open_btn.click()
                page.wait_for_timeout(5000)
                page.screenshot(path="/tmp/flow_2_after_click.png", full_page=True)
                print(f"Current URL: {page.url}")
        except Exception as e:
            print(f"Click error: {e}")

        # Wait and look for iframes
        page.wait_for_timeout(5000)
        frames = page.frames
        print(f"\n=== {len(frames)} frames after click ===")
        for i, f in enumerate(frames):
            print(f"  Frame[{i}]: {f.url}")

        openlist_frame = next((f for f in frames if "openlist" in f.url.lower()), None)
        if openlist_frame:
            print(f"\n=== OpenList frame: {openlist_frame.url} ===")
            page.wait_for_timeout(8000)
            page.screenshot(path="/tmp/flow_3_iframe.png", full_page=True)
            try:
                body_text = openlist_frame.locator("body").inner_text(timeout=10000)
                print(f"\nIframe body text (first 2000 chars):\n{body_text[:2000]}")
            except Exception as e:
                print(f"Body text error: {e}")
        else:
            print("\nNo openlist iframe found!")
            # Check current URL
            print(f"Current URL: {page.url}")

        # API requests
        print(f"\n=== API requests (api paths) ===")
        for entry in requests_log:
            url = entry.get("url", "")
            if "/api/" in url:
                if entry.get("type") == "request":
                    h = entry.get('headers', {})
                    interesting = {k: v for k, v in h.items() if k.lower() in ['content-type', 'origin']}
                    print(f"  REQ  {entry.get('method')} {url}")
                    if interesting: print(f"        {interesting}")
                elif entry.get("type") == "response":
                    h = entry.get('headers', {})
                    cors = {k: v for k, v in h.items() if k.lower().startswith('access-control')}
                    print(f"  RESP {entry.get('status')} {url}")
                    if cors: print(f"        cors: {cors}")

        # Console
        print(f"\n=== Console logs (last 30) ===")
        for log in console_logs[-30:]:
            print(f"  [{log['type']}] {log['text']}")

        # Errors
        print(f"\n=== Page errors ===")
        for err in page_errors:
            print(f"  {err}")

        with open("/tmp/flow_log.json", "w") as f:
            json.dump({"requests": requests_log, "console": console_logs, "page_errors": page_errors}, f, indent=2, default=str)

        browser.close()

if __name__ == "__main__":
    main()
