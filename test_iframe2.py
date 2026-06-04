"""Test the actual user flow in a real browser, but using 127.0.0.1:16000
(since the public IP 183.205.129.40 is NAT'd to the same host).
"""
from playwright.sync_api import sync_playwright
import json

URL = "http://127.0.0.1:16000/tabs/remote"

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
                })
            except Exception as e:
                pass

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
                pass

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

        page.wait_for_timeout(8000)

        page.screenshot(path="/tmp/page.png", full_page=True)
        print(f"Page screenshot: /tmp/page.png")

        try:
            print(f"Page title: {page.title()}")
        except: pass

        # Look for iframes
        frames = page.frames
        print(f"\n=== Found {len(frames)} frames ===")
        for i, frame in enumerate(frames):
            print(f"  Frame[{i}]: url={frame.url}")

        # Find openlist iframe
        openlist_frame = None
        for frame in frames:
            if "openlist" in frame.url.lower():
                openlist_frame = frame
                break

        if openlist_frame:
            print(f"\n=== OpenList frame: {openlist_frame.url} ===")
            page.wait_for_timeout(5000)
            page.screenshot(path="/tmp/with_iframe.png", full_page=True)

            try:
                body_text = openlist_frame.locator("body").inner_text(timeout=8000)
                print(f"\nIframe body text:\n{body_text[:2000]}")
            except Exception as e:
                print(f"Get iframe body error: {e}")

            try:
                html = openlist_frame.content()
                print(f"\nIframe HTML (first 2000 chars):\n{html[:2000]}")
            except Exception as e:
                print(f"Get iframe HTML error: {e}")
        else:
            print("\nNo openlist iframe found in main page")
            # Maybe the iframe is inside another element
            try:
                iframes = page.locator("iframe").all()
                print(f"  Found {len(iframes)} iframe elements")
                for i, iframe in enumerate(iframes):
                    src = iframe.get_attribute("src")
                    print(f"    iframe[{i}].src = {src}")
            except Exception as e:
                print(f"  Get iframes error: {e}")

        # Print all API requests
        print(f"\n=== API requests ===")
        for entry in requests_log:
            url = entry.get("url", "")
            if "/api/" in url or entry.get("type") == "response" and "/api/" in url:
                if entry.get("type") == "request":
                    print(f"  REQ  {entry.get('method')} {url}")
                    h = entry.get('headers', {})
                    interesting = {k: v for k, v in h.items() if k.lower() in
                                   ['origin', 'content-type', 'authorization', 'x-forwarded-prefix', 'host', 'referer']}
                    if interesting:
                        print(f"        headers: {interesting}")
                elif entry.get("type") == "response":
                    print(f"  RESP {entry.get('status')} {url}")
                    h = entry.get('headers', {})
                    interesting = {k: v for k, v in h.items() if k.lower().startswith('access-control') or
                                   k.lower() in ['content-type']}
                    if interesting:
                        print(f"        headers: {interesting}")

        # Print all console logs
        print(f"\n=== Console logs (last 50) ===")
        for log in console_logs[-50:]:
            print(f"  [{log['type']}] {log['text']}")

        # Print page errors
        print(f"\n=== Page errors ===")
        for err in page_errors:
            print(f"  {err}")

        # Find any failed requests
        failed_reqs = [r for r in requests_log if r.get("type") == "request" and
                       r.get("resource_type") in ("fetch", "xhr")]
        print(f"\n=== Fetch/XHR requests ({len(failed_reqs)}) ===")
        for r in failed_reqs[:30]:
            print(f"  {r.get('method')} {r.get('url')}")

        # Save log
        with open("/tmp/test2_log.json", "w") as f:
            json.dump({
                "requests": requests_log,
                "console": console_logs,
                "page_errors": page_errors,
            }, f, indent=2, default=str)

        browser.close()

if __name__ == "__main__":
    main()
