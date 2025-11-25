import argparse
import json
import time
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed

def run_request(args, request_id):
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {args.api_key}"
    }
    
    payload = {
        "model": args.model,
        "prompt": args.prompt
    }

    if args.path.endswith("/edits"):
        if not args.image_url:
            return request_id, 400, "Error: --image-url is required for edits endpoint", 0
        # For edits, we might need to download the image first or pass it as URL if supported.
        # The prompt implies "Use openai format", OpenAI edits usually take multipart/form-data with file.
        # However, the user said "specify image url", and the go code for edits seems to support both multipart and JSON with "images" array (custom).
        # But standard OpenAI edits endpoint is multipart.
        # Let's check the Go code again. 
        # The Go code `handleImageEdits` checks for multipart. If not multipart, it binds JSON.
        # If JSON, it expects `images` array.
        # So we can send JSON with "images": [url].
        payload["images"] = [args.image_url]
    
    url = f"{args.url.rstrip('/')}{args.path}"
    
    start_time = time.time()
    try:
        response = requests.post(url, headers=headers, json=payload, timeout=args.timeout)
        elapsed = time.time() - start_time
        return request_id, response.status_code, response.text, elapsed
    except Exception as e:
        elapsed = time.time() - start_time
        return request_id, 0, str(e), elapsed

def main():
    parser = argparse.ArgumentParser(description="Stress test script for Jimeng API")
    parser.add_argument("--concurrency", type=int, default=1, help="Number of concurrent requests")
    parser.add_argument("--total", type=int, default=1, help="Total number of requests")
    parser.add_argument("--url", type=str, default="http://localhost:8080", help="Base URL")
    parser.add_argument("--path", type=str, default="/v1/images/generations", help="API Path")
    parser.add_argument("--model", type=str, required=True, help="Model name")
    parser.add_argument("--api-key", type=str, required=True, help="API Key")
    parser.add_argument("--prompt", type=str, default="a cute cat", help="Prompt")
    parser.add_argument("--image-url", type=str, help="Image URL (required for /edits)")
    parser.add_argument("--timeout", type=int, default=30, help="Request timeout in seconds")

    args = parser.parse_args()

    print(f"Starting stress test with {args.concurrency} threads, {args.total} total requests...")
    print(f"Target: {args.url}{args.path} (Model: {args.model})")

    start_total = time.time()
    results = []
    
    with open("output.log", "w", encoding="utf-8") as log_file:
        with ThreadPoolExecutor(max_workers=args.concurrency) as executor:
            futures = [executor.submit(run_request, args, i) for i in range(args.total)]
            
            for future in as_completed(futures):
                req_id, status, body, elapsed = future.result()
                results.append((status, elapsed))
                
                log_msg = f"[Request {req_id}] Status: {status}, Time: {elapsed:.2f}s\nResponse: {body}\n{'-'*40}\n"
                print(f"[Request {req_id}] Status: {status}, Time: {elapsed:.2f}s")
                log_file.write(log_msg)
                log_file.flush()

                if status != 200:
                    print(f"  Response: {body[:200]}...") # Print first 200 chars of error

    total_time = time.time() - start_total
    success_count = sum(1 for r in results if r[0] == 200)
    avg_time = sum(r[1] for r in results) / len(results) if results else 0

    print("\n--- Test Summary ---")
    print(f"Total Requests: {args.total}")
    print(f"Successful: {success_count}")
    print(f"Failed: {args.total - success_count}")
    print(f"Total Time: {total_time:.2f}s")
    print(f"Avg Request Time: {avg_time:.2f}s")
    print(f"RPS: {args.total / total_time:.2f}")

if __name__ == "__main__":
    main()
