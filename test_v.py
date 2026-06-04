"""Verify the entire flow works after OpenPreview registered 2025.
Wait longer, retry on errors, capture iframe content.
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
        }))
        page.on("response", lambda resp: requests_log.append({
            "type": "response", "method": resp.request.method, "url": resp.url,
            "status": resp.status,
        }))
        page.on("console", lambda msg: console_logs.append({"type": msg.type, "text": msg.text}))
        page.on("pageerror", lambda err: page_errors.append(str(err)))

        print(f"Navigating to {URL} ...")
        page.goto(URL, timeout=60000, wait_until="networkidle")
        page.wait_for_timeout(5000)
        page.screenshot(path="/tmp/v1.png", full_page=True)
        print(f"After networkidle: {page.title()}")

        # Check if openlist tab loaded
        try:
            text = page.locator("body").inner_text(timeout=10000)
            print(f"\n=== Body text (first 1000 chars) ===\n{text[:1000]}")
        except Exception as e:
            print(f"body text error: {e}")

        # Find open webui button
        open_btn = page.locator(".open-webui-btn")
        cnt = open_btn.count()
        print(f"\n=== open-webui-btn count: {cnt} ===")

        if cnt > 0:
            print("Clicking open-webui-btn...")
            with page.expect_navigation(timeout=30000, wait_until="networkidle"):
                open_btn.first.click()
            page.wait_for_timeout(8000)
            page.screenshot(path="/tmp/v2.png", full_page=True)
            print(f"After click URL: {page.url}")
            print(f"Title: {page.title()}")

            # Find iframe
            frames = page.frames
            print(f"\n=== {len(frames)} frames ===")
            for i, f in enumerate(frames):
                print(f"  Frame[{i}]: {f.url}")

            openlist_frame = next((f for f in frames if "openlist" in f.url.lower() and "settings" not in f.url.lower()), None)
            if openlist_frame:
                print(f"\n=== OpenList iframe: {openlist_frame.url} ===")
                page.wait_for_timeout(10000)
                page.screenshot(path="/tmp/v3_iframe.png", full_page=True)
                try:
                    body_text = openlist_frame.locator("body").inner_text(timeout=15000)
                    print(f"\nIframe body text (first 3000 chars):\n{body_text[:3000]}")
                except Exception as e:
                    print(f"iframe body error: {e}")
        else:
            # Maybe local openlist is not running
            print("No open-webui-btn - check LocalOpenListStatusCard state")
            try:
                card = page.locator(".local-openlist-card").first
                if card.is_visible():
                    print(f"Card text: {card.text_content()[:500]}")
            except: pass

        # All API requests
        print(f"\n=== API requests (api paths) ===")
        for entry in requests_log:
            url = entry.get("url", "")
            if "/api/" in url and "encv" not in url:
                if entry.get("type") == "request":
                    print(f"  REQ  {entry.get('method')} {url}")
                elif entry.get("type") == "response":
                    print(f"  RESP {entry.get('status')} {url}")

        # Console
        print(f"\n=== Console logs (last 50) ===")
        for log in console_logs[-50:]:
            print(f"  [{log['type']}] {log['text']}")

        # Errors
        print(f"\n=== Page errors ===")
        for err in page_errors:
            print(f"  {err}")

        # Failed network requests
        failed = [r for r in requests_log if r.get("type") == "response" and r.get("status", 200) >= 400]
        print(f"\n=== Failed responses ({len(failed)}) ===")
        for f in failed[:30]:
            print(f"  {f.get('status')} {f.get('url')}")

        with open("/tmp/v_log.json", "w") as f:
            json.dump({"requests": requests_log, "console": console_logs, "page_errors": page_errors}, f, indent=2, default=str)

        browser.close()

if __name__ == "__main__":
    main()
