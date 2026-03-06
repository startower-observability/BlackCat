---
name: Veo 3 Video Generation
tags: [video, ai, generation, google, veo]
requires:
  bins: [uv, ffmpeg]
  env: [GEMINI_API_KEY]
---

# Veo 3 Video Generation

Generate videos from text prompts using Google's Veo 3 model via the Gemini API.

## Prerequisites

- `uv` (Python package manager)
- `ffmpeg` (for video processing)
- `GEMINI_API_KEY` environment variable

## Capabilities

- Generate videos from text prompts
- Control duration (8 seconds standard, up to 8 seconds for Veo 3)
- Specify aspect ratio (16:9, 9:16, 1:1)
- Check generation progress
- Download and save results

## How to Use

### Quick video generation

```python
import os
import requests
import time

api_key = os.environ["GEMINI_API_KEY"]
prompt = "A serene mountain lake at sunrise with mist rolling over the water"

# Initialize generation
response = requests.post(
    f"https://generativelanguage.googleapis.com/v1beta/models/veo-3-generate-preview:generateContent?key={api_key}",
    json={
        "contents": [{
            "parts": [{"text": prompt}]
        }],
        "generationConfig": {
            "responseModalities": ["VIDEO"]
        }
    }
).json()

operation_name = response.get("name")
print(f"Operation: {operation_name}")

# Poll for completion
while True:
    result = requests.get(
        f"https://generativelanguage.googleapis.com/v1beta/{operation_name}?key={api_key}"
    ).json()
    
    if "done" in result and result["done"]:
        video_uri = result["response"]["candidates"][0]["content"]["parts"][0]["fileData"]["uri"]
        print(f"Video ready: {video_uri}")
        break
    
    print("Generating...")
    time.sleep(10)
```

### Download the video

```python
import requests

video_uri = "your_video_uri_here"
file_name = video_uri.split("/")[-1]

# Get download URL
download_response = requests.get(
    f"https://generativelanguage.googleapis.com/v1beta/{file_name}?key={api_key}"
).json()

# Download video
video_url = download_response["uri"]
video_data = requests.get(video_url).content

with open("output.mp4", "wb") as f:
    f.write(video_data)

print("Saved to output.mp4")
```

### With parameters (aspect ratio, duration hints)

```python
response = requests.post(
    f"https://generativelanguage.googleapis.com/v1beta/models/veo-3-generate-preview:generateContent?key={api_key}",
    json={
        "contents": [{"parts": [{"text": prompt}]}],
        "generationConfig": {
            "responseModalities": ["VIDEO"],
            "aspectRatio": "16:9"  # or "9:16", "1:1"
        }
    }
).json()
```

## Error Handling

Common errors and solutions:

- **429 Too Many Requests**: Rate limit hit. Wait 60 seconds and retry.
- **Quota exceeded**: Daily limit reached. Check console.cloud.google.com
- **Invalid prompt**: Content policy violation. Rephrase without restricted content.
- **Generation failed**: Retry with simpler prompt or shorter duration.

## Notes

- Generation takes 30-120 seconds depending on complexity
- Max prompt length: 2000 characters
- Default output: 1280x720 (16:9), 8 seconds, 24fps
- Videos expire after 48 hours — download immediately
- Supports negative prompts via "Avoid: " prefix in prompt text
