---
name: xsearch
description: >
  Search X (Twitter) in real-time using the xAI Grok API. Activate when the
  user wants to find posts, trends, discussions, or what specific people are
  saying on X. Also activate when the user needs recent public sentiment,
  breaking news reactions, or social media context on any topic.
license: MIT
metadata:
  author: chonkskill
  version: "1.0"
  requires_env:
    - XAI_API_KEY
  requires_tools:
    - xsearch:search
    - xsearch:search_with_media
    - xsearch:profile
---

# X (Twitter) Search

You have real-time search access to X (Twitter) through the xAI Grok API.
Grok has native access to the full X firehose -- it searches posts, analyzes
trends, and summarizes discussions with source citations.

Results always include direct links to the original X posts.

---

## Quick Reference

| Task | Tool | Example |
|------|------|---------|
| Search any topic | `xsearch:search` | "AI regulation news" |
| Search with image/video analysis | `xsearch:search_with_media` | "infographics about climate data" with images=true |
| Search a specific user's posts | `xsearch:profile` | handle: "elonmusk" |
| Date-bounded search | `xsearch:search` | from_date: "2026-03-01", to_date: "2026-03-28" |
| Filter to specific accounts | `xsearch:search` | handles: ["openai", "anthropic"] |
| Exclude noisy accounts | `xsearch:search` | exclude_handles: ["spambot"] |

---

## When to Search X

Search X when:

- The user asks what people are saying about a topic
- The user wants breaking news or recent developments
- The user asks about trending topics or public sentiment
- The user wants to know what a specific person posted on X
- The user needs social media context for a news story or event
- The user asks about community reactions, discourse, or debates
- The user references X, Twitter, tweets, or posts

Do not search X when:

- The user needs factual reference information (use web search instead)
- The question is about historical facts with no social media angle
- The user explicitly wants only authoritative sources

---

## Search Strategy

### General topic searches

1. Start with a clear, specific query: `xsearch:search` with query "topic"
2. Review the results and citations
3. If results are too broad, add date filters or handle filters
4. If results miss the mark, rephrase the query

### Person-specific searches

Use `xsearch:profile` when the user asks about a specific person's posts:

1. `xsearch:profile` with handle "username"
2. Optionally add a topic filter with the query param
3. Add date filters to narrow to recent posts

### Trend and sentiment analysis

1. Search the topic broadly: `xsearch:search` with the topic
2. The results from Grok include synthesis and analysis of the discourse
3. Citations link to representative posts

### Media-rich searches

Use `xsearch:search_with_media` when:

- The user wants to find posts with images (infographics, screenshots, charts)
- The user wants video content analyzed
- The search topic is visual in nature (product launches, events, design)

Enable `images: true` and/or `video: true` as needed. These increase token
usage so only enable when the user's query benefits from media analysis.

---

## Presenting Results

When presenting X search results:

- Lead with the key findings -- what is the consensus, trend, or answer
- Include 2-5 of the most relevant citations with their X post URLs
- Note the date range of the results if time-sensitive
- Flag if results show conflicting viewpoints and present both sides
- If the user asked about a specific person, quote or summarize their posts

### Citation format

Always include X post URLs from the citations. Format as:

> Key finding or quote
> -- @handle ([source](https://x.com/...))

---

## Filtering

### Handle filters

- `handles`: Show ONLY posts from these accounts (max 10)
- `exclude_handles`: Show posts from everyone EXCEPT these accounts (max 10)
- These two filters are mutually exclusive -- you cannot use both

Handle filters are useful when:

- The user asks what a specific person or company said
- The user wants to filter out known spam or noise accounts
- The user wants posts only from domain experts or official accounts

### Date filters

- `from_date`: Posts on or after this date (YYYY-MM-DD)
- `to_date`: Posts on or before this date (YYYY-MM-DD)
- Both are optional and can be used independently

Date filters are useful when:

- The user asks about "recent" posts (use last 7 days)
- The user references a specific event date
- The user wants to track sentiment over a time period

---

## Pitfalls

- **Handle names are case-insensitive** but must not include the @ symbol.
  Strip it if the user provides "@elonmusk" -- pass "elonmusk".

- **handles and exclude_handles are mutually exclusive.** If the user wants
  to see posts from A but not B, use handles with just A, not both filters.

- **Grok synthesizes results.** The text response is Grok's summary, not
  raw post text. The citations link to the actual posts. Always include
  citations so the user can verify.

- **Rate limits apply.** The xAI API has rate limits. If a search fails,
  wait before retrying.

- **Media analysis costs more tokens.** Only enable image/video understanding
  when the query actually needs it.
