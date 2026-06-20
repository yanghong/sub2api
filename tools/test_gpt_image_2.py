#!/usr/bin/env python3
"""Probe a Sub2API/OpenAI-compatible online gateway for gpt-image-2 support.

The script intentionally uses only the Python standard library so it can run
from a production jump host without installing SDK dependencies.
"""

from __future__ import annotations

import argparse
import base64
import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


DEFAULT_IMAGE_PROMPT = (
    "A clean product poster for a translucent smart speaker on a desk, "
    "realistic studio lighting, readable title text: GPT IMAGE 2 TEST"
)

DEFAULT_RESPONSES_PROMPT = (
    "Generate a 16:9 realistic SaaS dashboard launch visual. Include readable "
    "headline text: GPT IMAGE 2 RESPONSES TEST."
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Test gpt-image-2 image generation through an online gateway.",
    )
    parser.add_argument(
        "--base-url",
        default=os.getenv("SUB2API_BASE_URL", "").rstrip("/"),
        help="Gateway base URL, e.g. https://api.example.com. Env: SUB2API_BASE_URL",
    )
    parser.add_argument(
        "--api-key",
        default=os.getenv("SUB2API_API_KEY", ""),
        help="Bearer API key. Env: SUB2API_API_KEY",
    )
    parser.add_argument(
        "--image-model",
        default=os.getenv("SUB2API_IMAGE_MODEL", "gpt-image-2"),
        help="Image model to test. Env: SUB2API_IMAGE_MODEL",
    )
    parser.add_argument(
        "--responses-model",
        default=os.getenv("SUB2API_RESPONSES_MODEL", "gpt-5.4"),
        help="Text-capable Responses model used to call image_generation. Env: SUB2API_RESPONSES_MODEL",
    )
    parser.add_argument(
        "--out-dir",
        default=os.getenv("SUB2API_IMAGE_TEST_OUT", "tmp/gpt-image-2-test"),
        help="Directory for response JSON and generated images.",
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=int(os.getenv("SUB2API_IMAGE_TEST_TIMEOUT", "180")),
        help="Request timeout in seconds.",
    )
    parser.add_argument(
        "--skip-responses",
        action="store_true",
        help="Only test /v1/images/generations.",
    )
    parser.add_argument(
        "--image-prompt",
        default=os.getenv("SUB2API_IMAGE_PROMPT", DEFAULT_IMAGE_PROMPT),
        help="Prompt for /v1/images/generations.",
    )
    parser.add_argument(
        "--responses-prompt",
        default=os.getenv("SUB2API_RESPONSES_PROMPT", DEFAULT_RESPONSES_PROMPT),
        help="Prompt for /v1/responses image_generation.",
    )
    return parser.parse_args()


def request_json(
    *,
    base_url: str,
    api_key: str,
    path: str,
    payload: dict[str, Any],
    timeout: int,
) -> tuple[int, dict[str, str], Any]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        f"{base_url}{path}",
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        },
    )

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            return resp.status, dict(resp.headers), decode_json_or_text(raw)
    except urllib.error.HTTPError as err:
        raw = err.read()
        return err.code, dict(err.headers), decode_json_or_text(raw)


def decode_json_or_text(raw: bytes) -> Any:
    text = raw.decode("utf-8", errors="replace")
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return {"raw": text}


def save_json(path: Path, data: Any) -> None:
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2), encoding="utf-8")


def write_image_from_b64(path: Path, b64_value: str) -> int:
    if b64_value.startswith("data:"):
        _, b64_value = b64_value.split(",", 1)
    data = base64.b64decode(b64_value)
    path.write_bytes(data)
    return len(data)


def extract_images_from_image_api(response: Any) -> list[tuple[str, str]]:
    images: list[tuple[str, str]] = []
    for idx, item in enumerate((response or {}).get("data") or []):
        if item.get("b64_json"):
            images.append((f"images-api-{idx}.png", item["b64_json"]))
        elif isinstance(item.get("url"), str) and item["url"].startswith("data:image/"):
            images.append((f"images-api-{idx}.png", item["url"]))
    return images


def extract_images_from_responses_api(response: Any) -> list[tuple[str, str]]:
    images: list[tuple[str, str]] = []
    for idx, item in enumerate((response or {}).get("output") or []):
        if item.get("type") == "image_generation_call" and item.get("result"):
            images.append((f"responses-api-{idx}.png", item["result"]))
    return images


def summarize(label: str, status: int, response: Any, saved: list[Path], elapsed: float) -> None:
    ok = 200 <= status < 300 and bool(saved)
    usage = response.get("usage") if isinstance(response, dict) else None
    error = response.get("error") if isinstance(response, dict) else None
    print(f"\n[{label}] status={status} ok={ok} elapsed={elapsed:.1f}s")
    if usage:
        print(f"  usage={json.dumps(usage, ensure_ascii=False)}")
    if saved:
        for path in saved:
            print(f"  image={path}")
    if error:
        print(f"  error={json.dumps(error, ensure_ascii=False)}")
    if not saved and not error:
        print("  no image found in response; inspect saved JSON for details")


def run_images_generation(args: argparse.Namespace, out_dir: Path) -> bool:
    payload = {
        "model": args.image_model,
        "prompt": args.image_prompt,
        "size": "1024x1024",
        "quality": "medium",
        "output_format": "png",
        "response_format": "b64_json",
        "n": 1,
    }
    started = time.monotonic()
    status, _, response = request_json(
        base_url=args.base_url,
        api_key=args.api_key,
        path="/v1/images/generations",
        payload=payload,
        timeout=args.timeout,
    )
    elapsed = time.monotonic() - started

    save_json(out_dir / "images-api-response.json", response)
    saved = []
    for name, b64_value in extract_images_from_image_api(response):
        path = out_dir / name
        size = write_image_from_b64(path, b64_value)
        if size > 0:
            saved.append(path)
    summarize("images.generations", status, response, saved, elapsed)
    return 200 <= status < 300 and bool(saved)


def run_responses_image_generation(args: argparse.Namespace, out_dir: Path) -> bool:
    payload = {
        "model": args.responses_model,
        "input": args.responses_prompt,
        "tools": [
            {
                "type": "image_generation",
                "model": args.image_model,
                "size": "2048x1152",
                "quality": "medium",
                "output_format": "png",
            }
        ],
        "tool_choice": {"type": "image_generation"},
    }
    started = time.monotonic()
    status, _, response = request_json(
        base_url=args.base_url,
        api_key=args.api_key,
        path="/v1/responses",
        payload=payload,
        timeout=args.timeout,
    )
    elapsed = time.monotonic() - started

    save_json(out_dir / "responses-api-response.json", response)
    saved = []
    for name, b64_value in extract_images_from_responses_api(response):
        path = out_dir / name
        size = write_image_from_b64(path, b64_value)
        if size > 0:
            saved.append(path)
    summarize("responses.image_generation", status, response, saved, elapsed)
    return 200 <= status < 300 and bool(saved)


def main() -> int:
    args = parse_args()
    if not args.base_url:
        print("missing --base-url or SUB2API_BASE_URL", file=sys.stderr)
        return 2
    if not args.api_key:
        print("missing --api-key or SUB2API_API_KEY", file=sys.stderr)
        return 2

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    print(f"base_url={args.base_url}")
    print(f"image_model={args.image_model}")
    print(f"responses_model={args.responses_model}")
    print(f"out_dir={out_dir}")

    results = [run_images_generation(args, out_dir)]
    if not args.skip_responses:
        results.append(run_responses_image_generation(args, out_dir))

    passed = sum(1 for item in results if item)
    print(f"\nsummary: {passed}/{len(results)} probes produced an image")
    return 0 if all(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
