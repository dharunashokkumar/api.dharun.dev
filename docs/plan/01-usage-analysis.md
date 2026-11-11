# Usage analysis: 14 October to 10 November 2026

Read on 2026-11-10 from the private monitoring (the API's own Prometheus metrics, the
same data the Grafana dashboard shows). 27 days, from the first production day.

**Not available for this analysis:** the request logs (which tickers were missing, which
user agents, the `error` field) and Google Search Console (queries, impressions, indexed
pages). Both would sharpen the conclusions; section 6 lists what to look up.

## 1. Size

| Measure | Value |
|---|---|
| Requests, health checks left out | about 262,000 |
| Unique visitors per day | 300 to 490 (459 on the last day) |
| Unique visitors, last full month / this month so far | 5,677 / 3,789 |
| Median response time | 1 ms |
| 95th / 99th percentile | 20 ms / 363 ms |

A normal day has 2,000 to 6,000 requests. Days above 10,000 are each caused by one
caller (section 3).

## 2. What is used

Requests in 30 days by route, top of the list:

| Route | Requests | Share | Pattern |
|---|---|---|---|
| Logos | 145,750 | 56% | Every day, 400 to 31,000 a day, mostly browsers |
| Phone numbers | 34,890 | 13% | Two days: 5,400 on 19 Oct, 28,900 on 5 Nov |
| MCP | 30,240 | 12% | Flat, about 1,100 a day |
| Docs pages | 12,210 | 5% | About 400 a day |
| Email check | 11,130 | 4% | Two days: 5 and 6 Nov |
| Panchang | 7,105 | 3% | 890 on 30 Oct, 6,050 on 10 Nov |
| Refused by the rate limit | 6,970 | 3% | Four days, the same days as logo spikes |
| Companies (lookup and search) | 5,220 | 2% | Every day |
| Translation | 2,620 | 1% | One day (3 Nov) |
| Pincodes | 750 | | 545 of them on one day |
| OCR | 727 | | One day (8 Nov) |

Everything else, about 60 endpoints, had **fewer than 50 requests each** in 30 days:
IFSC 36, UPI QR 36, working days 31, redaction 30, whois 23, HSN 17, typst 14, embed 8,
TLS 7, invoice 6.

Logo answers: 119,000 found, 23,000 not found (16%), 3,900 monograms on request. Two in
three are SVG.

## 3. Who sends the requests

| Kind | Requests | Notes |
|---|---|---|
| Unknown (no user agent) | 89,400 | Almost all from two batch runs on 5 and 6 Nov |
| Person (browser) | 81,200 | Mostly logo images loading inside someone's web app |
| Program | 42,700 | Python 20,500, Node 9,500, Java 5,900, curl 2,500, Dart 1,800, Bun 500 |
| Bot | 38,800 | MCP directory probes 10,800, headless browsers 10,500, others |
| AI assistant | 1,770 | Cursor 907, Claude 814, Copilot 28, ChatGPT 19 |
| AI crawler | 927 | Amazonbot 456, GPTBot 223, Meta 154, ClaudeBot 45, OAI-SearchBot 27 |
| Search crawler | 631 | Applebot 446, Bingbot 116, **Googlebot 61** |

- **Country:** India 72% of requests (Maharashtra and Delhi 45,000 each, Andhra Pradesh
  27,000, Rajasthan 21,000), United States 11%. By unique visitors per day it is the
  other way round (about 190 from the US against about 25 from India): many foreign
  visitors are bots making few requests, and a small number of Indian users make most of
  the requests.
- **Arrived from Google:** 1,063 requests in 30 days, in two short bursts.
- **Followed one of our own tagged links:** docs 191, skill 74, guides 72, `/v1` index 55,
  `llms.txt` 16, home page 2.
- **MCP:** of 30,000 requests, 28,000 are directory probes, bots and unnamed programs.
  2,578 were answered 405 (clients trying GET on a POST-only address).

## 4. Faults seen

| Fault | Numbers | State |
|---|---|---|
| Panchang answers 500 | 1,007 on 10 Nov (of about 6,000), 19 on 30 Oct | **Open, cause unknown.** 25 edge cases tried against the live site (latitude limits, year limits, pincodes, date line) all answered correctly. The log line's `error` field names the cause |
| Logo not found | 22,955 | Addressed in Part 1 |
| Rate limit refusals | 6,967 | Days with logo spikes: an app with many users behind one address, or one scraper. Addressed in Parts 1 and 2 |
| MCP 405 | 2,578 | Probably directory checkers sending GET. Worth one look in the logs |
| Company not found | 981 | Same cause as logos |

## 5. What follows

1. **Logos are the product people came for.** They are the only API with daily use from
   real applications. Work on them first: fewer misses, more coverage, no broken images.
2. **Other use is batch work.** Someone ran 29,000 phone numbers in a day, someone else
   11,000 email addresses, someone 2,600 translations. These callers want bulk
   endpoints and higher limits, which is what batch endpoints (05, set A) and keys (03)
   give them.
3. **Reach, not the number of endpoints, limits growth.** About 60 working endpoints are
   close to unused. Google has fetched 61 pages in a month. The tool pages and try-it
   panels (04) make the existing endpoints findable and usable; the launch stays the
   biggest single lever and is on hold by decision.
4. **Flutter and Android developers are already here** (Dart 1,800, Java 5,900
   requests). A Dart package and Flutter and Kotlin guides would meet them.
5. **Assistants call the API, but rarely.** Cursor and Claude together made 1,700
   requests. The MCP numbers on the dashboard overstate real use by about fifteen times.
6. **Speed is not a problem.** The slow routes are slow for a reason: OCR 4.7 s,
   embeddings and reranking 2.4 s (the model helper starting cold), domain icons 2.1 s (a
   fetch from the site), traceroute 9.8 s (waiting on the network). The one cheap gain
   is keeping the small embedding model loaded.

## 6. To look up before Part 1

From the logs (reached from the homelab side):

- The 100 most-requested logo symbols that were not found. This decides which of the
  three new logo sets matters most and whether callers send names, tickers or
  something else (`RELIANCE.NS`, `NSE:RELIANCE`, lower case, company names).
- The `error` and path of the panchang 500s.
- The referring sites and user agents behind the logo traffic from browsers (which apps
  embed the logos), and whether the 429s come from one network.
- The user agents behind the phone and email batch runs.

From Search Console and Bing Webmaster Tools:

- Pages indexed against pages in the sitemap, and any crawl errors.
- Queries with impressions, and the pages they land on.
- Bing's AI Performance report, if the property has it.
