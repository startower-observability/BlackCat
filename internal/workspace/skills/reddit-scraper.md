---
name: Reddit Scraper
tags: [reddit, scraping, social, api]
requires:
  bins: [python3]
---

# Reddit Scraper

Scrape Reddit posts, comments, and search results using Reddit's public JSON API. No API key required.

## Prerequisites

- `python3` binary
- `requests` library: `pip install requests`

## Capabilities

- Fetch subreddit posts (hot, new, top, rising)
- Get post comments and replies
- Search Reddit posts
- Extract post metadata (score, author, timestamp, etc.)
- Handle pagination for large datasets

## How to Use

### Fetch subreddit posts

```python
import requests
import time

headers = {"User-Agent": "blackcat-scraper/1.0"}

# Get hot posts from a subreddit
url = "https://www.reddit.com/r/programming/hot.json?limit=25"
response = requests.get(url, headers=headers)
data = response.json()

for post in data["data"]["children"]:
    p = post["data"]
    print(f"{p['score']}: {p['title'][:80]}")
    print(f"   https://reddit.com{p['permalink']}")
    print()
```

### Get post comments

```python
import requests

headers = {"User-Agent": "blackcat-scraper/1.0"}
permalink = "/r/programming/comments/abc123/post_title"

url = f"https://www.reddit.com{permalink}.json"
response = requests.get(url, headers=headers)
data = response.json()

# data[0] is the post, data[1] is comments
post = data[0]["data"]["children"][0]["data"]
comments = data[1]["data"]["children"]

print(f"Post: {post['title']}")
print(f"Score: {post['score']}")
print("\nComments:")

for comment in comments[:10]:
    if comment["kind"] == "t1":  # t1 = comment
        c = comment["data"]
        print(f"  {c['author']}: {c['body'][:100]}...")
```

### Search Reddit

```python
import requests
import urllib.parse

headers = {"User-Agent": "blackcat-scraper/1.0"}
query = urllib.parse.quote("python tutorial")

url = f"https://www.reddit.com/search.json?q={query}&sort=relevance&limit=25"
response = requests.get(url, headers=headers)
data = response.json()

for post in data["data"]["children"]:
    p = post["data"]
    print(f"r/{p['subreddit']}: {p['title'][:80]}")
```

### Paginate results

```python
import requests
import time

headers = {"User-Agent": "blackcat-scraper/1.0"}
after = None
all_posts = []

for page in range(5):  # Get 5 pages
    url = f"https://www.reddit.com/r/technology/new.json?limit=100"
    if after:
        url += f"&after={after}"
    
    response = requests.get(url, headers=headers)
    data = response.json()
    
    posts = data["data"]["children"]
    all_posts.extend(posts)
    
    after = data["data"]["after"]
    if not after:
        break
    
    time.sleep(2)  # Rate limit: 2 second delay between requests

print(f"Total posts collected: {len(all_posts)}")
```

## Rate Limiting

Reddit's public API has unauthenticated rate limits:

- **Rule**: 1 request per 2 seconds recommended
- **User-Agent required**: Always include a descriptive User-Agent header
- **429 response**: If hit, wait 60 seconds before retrying
- **IP-based limits**: Shared across all requests from your IP

```python
import time

def fetch_with_backoff(url, headers, max_retries=3):
    for attempt in range(max_retries):
        response = requests.get(url, headers=headers)
        
        if response.status_code == 200:
            return response
        elif response.status_code == 429:
            wait_time = (attempt + 1) * 10
            print(f"Rate limited. Waiting {wait_time}s...")
            time.sleep(wait_time)
        else:
            response.raise_for_status()
    
    raise Exception("Max retries exceeded")
```

## Output Format

Post object structure:

```python
{
    "title": "Post title",
    "author": "username",
    "subreddit": "subreddit_name",
    "score": 1234,
    "num_comments": 56,
    "created_utc": 1234567890.0,
    "permalink": "/r/subreddit/comments/id/title",
    "url": "https://example.com",  # External link or selftext
    "selftext": "Post body text",  # For text posts
    "thumbnail": "url_or_default"
}
```

## Notes

- Public API returns ~25 listings max per request
- Sort options: hot, new, top, rising, controversial
- Time filters for top: hour, day, week, month, year, all
- Comments can be deeply nested — recurse for full threads
- Some subreddits may be private or quarantined (403/404)
- Respect robots.txt and Reddit's Terms of Service
