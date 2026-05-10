
import asyncio
import json
import sys
from playwright.async_api import async_playwright

async def capture_screenshot(url, output_path, width):
    try:
        async with async_playwright() as p:
            browser = await p.chromium.launch(headless=True)
            page = await browser.new_page()
            await page.set_viewport_size({"width": width, "height": 800})
            await page.goto(url)
            await page.screenshot(path=output_path, full_page=True)
            await browser.close()
        return {"success": True, "path": output_path}
    except Exception as e:
        return {"success": False, "error": str(e)}

if __name__ == "__main__":
    if len(sys.argv) != 4:
        print(json.dumps({"success": False, "error": "Usage: python vqa_capture.py <URL> <output_path> <width>"}))
        sys.exit(1)

    url = sys.argv[1]
    output_path = sys.argv[2]
    width = int(sys.argv[3])

    result = asyncio.run(capture_screenshot(url, output_path, width))
    print(json.dumps(result))
