# Smoke WASM: abre la pagina de prueba Pyodide en Chromium headless y espera
# WASM_RESULT (exito) o WASM_ERROR. Requiere el http.server sirviendo
# D:\PortableSatelital en 8787.
import json
import sys

from playwright.sync_api import sync_playwright

URL = "http://127.0.0.1:8787/dist/pyodide-test/index.html"
TIMEOUT_MS = 900_000  # primera carga baja ~100 MB de paquetes del CDN


def main():
    result = {}
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        done = {"status": None, "payload": None}

        def on_console(msg):
            text = msg.text
            print("  [browser]", text[:160])
            if text.startswith("WASM_RESULT:"):
                done["status"] = "ok"
                done["payload"] = text[len("WASM_RESULT:"):]
            elif text.startswith("WASM_ERROR:"):
                done["status"] = "error"
                done["payload"] = text

        page.on("console", on_console)
        page.goto(URL)
        page.wait_for_function("() => window.__done === true", timeout=0) \
            if False else None
        # espera activa por el marcador en consola
        import time
        t0 = time.time()
        while done["status"] is None and (time.time() - t0) * 1000 < TIMEOUT_MS:
            page.wait_for_timeout(500)
        browser.close()

    if done["status"] == "ok":
        result = json.loads(done["payload"])
        print("\n=== SEGMENTACION SHEPHERD EN EL NAVEGADOR: OK ===")
        for k, v in result.items():
            print(f"  {k}: {v}")
        return 0
    print("\n=== FALLO ===")
    print(done["payload"])
    return 1


if __name__ == "__main__":
    sys.exit(main())
