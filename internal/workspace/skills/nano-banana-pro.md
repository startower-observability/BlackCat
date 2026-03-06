---
name: Nano Banana Pro
tags: [image, ai, generation, gemini, editing]
requires:
  bins: [uv]
  env: [GEMINI_API_KEY]
---

# Nano Banana Pro (Gemini Image Generation)

Generate and edit images using Gemini Pro's native image generation capabilities.

## Prerequisites

- `uv` (Python package manager)
- `GEMINI_API_KEY` environment variable

## Capabilities

- Text-to-image generation
- Image editing with prompts (inpainting/outpainting)
- Multiple output formats (PNG, JPEG)
- Various aspect ratios and resolutions

## How to Use

### Generate image from text

```python
import os
import requests
import base64

api_key = os.environ["GEMINI_API_KEY"]
prompt = "A futuristic cityscape with flying cars at sunset, cyberpunk style"

response = requests.post(
    f"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-exp-image-generation:generateContent?key={api_key}",
    json={
        "contents": [{
            "parts": [{"text": prompt}]
        }],
        "generationConfig": {
            "responseModalities": ["IMAGE", "TEXT"]
        }
    }
).json()

# Extract image data
for part in response["candidates"][0]["content"]["parts"]:
    if "inlineData" in part:
        image_data = base64.b64decode(part["inlineData"]["data"])
        with open("generated.png", "wb") as f:
            f.write(image_data)
        print("Saved to generated.png")
```

### Edit an existing image

```python
import base64

# Read existing image
with open("input.png", "rb") as f:
    image_b64 = base64.b64encode(f.read()).decode()

response = requests.post(
    f"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-exp-image-generation:generateContent?key={api_key}",
    json={
        "contents": [{
            "parts": [
                {"text": "Change the sky to a starry night and add a full moon"},
                {
                    "inlineData": {
                        "mimeType": "image/png",
                        "data": image_b64
                    }
                }
            ]
        }],
        "generationConfig": {
            "responseModalities": ["IMAGE", "TEXT"]
        }
    }
).json()

# Save edited image
for part in response["candidates"][0]["content"]["parts"]:
    if "inlineData" in part:
        with open("edited.png", "wb") as f:
            f.write(base64.b64decode(part["inlineData"]["data"]))
```

### Batch generation with variations

```python
prompts = [
    "A red apple on a wooden table, photorealistic",
    "The same red apple, but sliced in half showing seeds",
    "The apple tree in an orchard, golden hour lighting"
]

for i, prompt in enumerate(prompts):
    response = requests.post(
        f"https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-exp-image-generation:generateContent?key={api_key}",
        json={
            "contents": [{"parts": [{"text": prompt}]}],
            "generationConfig": {"responseModalities": ["IMAGE"]}
        }
    ).json()
    
    for part in response["candidates"][0]["content"]["parts"]:
        if "inlineData" in part:
            with open(f"variation_{i}.png", "wb") as f:
                f.write(base64.b64decode(part["inlineData"]["data"]))
```

## Rate Limits

- Free tier: 15 requests per minute, 1500 per day
- Paid tier: 60 requests per minute (adjust based on your plan)
- Burst limit: 5 concurrent requests

## Supported Formats

- Input: PNG, JPEG, WEBP, HEIC, HEIF
- Output: PNG (default), JPEG
- Max input size: 20MB per image
- Max output resolution: 2048x2048

## Error Handling

```python
if "error" in response:
    error_code = response["error"]["code"]
    if error_code == 429:
        print("Rate limit exceeded. Wait 60 seconds.")
    elif error_code == 400:
        print("Invalid request. Check prompt or image format.")
    elif error_code == 403:
        print("API key invalid or quota exceeded.")
```

## Notes

- Add "photorealistic" or "digital art" to control style
- Use "no text" or "no words" in prompt to avoid garbled text
- For consistent characters, describe detailed features
- Image editing works best with clear, high-quality inputs
