#!/usr/bin/env python3
"""
One clean proof run: POST /api/v1/chat, X-LLM-Provider: local-worker, capture HTTP 200 + body.

Run from repo root. Use a long client timeout (sandbox CPU can need 10–15 min for first reply).

Usage:
    python3 scripts/proof_local_worker_chat.py [apiserver_url]
    python3 scripts/proof_local_worker_chat.py http://127.0.1.6:8080

Exit codes:
    0: HTTP 200 with real response body
    1: Non-200 response or error
"""

import sys
import os
import time
import urllib.request
import urllib.error
import json

# Configuration
BASE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.1.6:8080"
OUT = "/tmp/proof_response.txt"
CURL_TIMEOUT = 1500  # Long timeout for sandbox Ollama (warmup + first reply can be 10–15 min)

def main():
    # Optional: wait so apiserver warmup can complete before sending (deterministic warm path)
    wait_sec = os.environ.get("PROOF_WAIT_WARMUP_SECONDS")
    if wait_sec:
        try:
            s = int(wait_sec)
            if s > 0:
                print(f"Waiting {s}s for warmup to complete...")
                time.sleep(s)
        except ValueError:
            pass
    print(f"Proof: POST {BASE}/api/v1/chat (X-LLM-Provider: local-worker), timeout={CURL_TIMEOUT}s")

    # Clean output file
    if os.path.exists(OUT):
        os.remove(OUT)

    # Prepare request
    url = f"{BASE}/api/v1/chat"
    data = {
        "messages": [
            {"role": "user", "content": "Reply with exactly: ok"}
        ]
    }

    headers = {
        "Content-Type": "application/json",
        "X-LLM-Provider": "local-worker"
    }

    req = urllib.request.Request(
        url,
        data=json.dumps(data).encode('utf-8'),
        headers=headers,
        method='POST'
    )

    # Execute request
    start_time = time.time()
    try:
        with urllib.request.urlopen(req, timeout=CURL_TIMEOUT) as response:
            body = response.read().decode('utf-8')
            http_code = response.status
            time_total = time.time() - start_time

            # Write output
            output = f"{body}\n\nHTTP_CODE:{http_code}\nTIME_TOTAL:{time_total:.2f}s\n"
            with open(OUT, 'w') as f:
                f.write(output)

            print("---BODY + FOOTER---")
            print(output)

            if http_code == 200:
                print("SUCCESS: HTTP 200 with real response body.")
                return 0
            else:
                print(f"Not 200; check body above and apiserver/Ollama logs.")
                return 1

    except urllib.error.HTTPError as e:
        time_total = time.time() - start_time
        body = e.read().decode('utf-8')
        output = f"{body}\n\nHTTP_CODE:{e.code}\nTIME_TOTAL:{time_total:.2f}s\n"
        with open(OUT, 'w') as f:
            f.write(output)

        print("---BODY + FOOTER---")
        print(output)
        print(f"Not 200; check body above and apiserver/Ollama logs.")
        return 1

    except Exception as e:
        print(f"ERROR: {e}")
        return 1

if __name__ == "__main__":
    sys.exit(main())
