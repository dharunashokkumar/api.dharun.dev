---
title: Show NSE and BSE company logos in your app
description: Put a listed company's logo in an img tag by ticker or ISIN, as SVG or sized PNG, with an initials tile when no logo exists.
apis: logos
published: 2026-10-14
---

A watchlist or portfolio screen reads much faster with logos. The logo endpoint returns the image itself, so the URL goes straight into an `<img>` tag.

## By ticker

```html
<img src="https://api.dharun.dev/v1/logos/TCS" alt="TCS" width="32" height="32">
```

A symbol can be a bare ticker (`TCS`, NSE is preferred), an exchange-qualified ticker (`NSE:TCS`), a Yahoo Finance symbol (`TCS.NS`) or an ISIN (`INE467B01029`).

## PNG at a set size

SVG is the default. For PNG, give `format=png` and a `size` from 16 to 512 pixels:

```html
<img src="https://api.dharun.dev/v1/logos/TCS?format=png&size=128" alt="TCS" width="64" height="64">
```

Asking for twice the size you display keeps it sharp on high-density screens.

## When there is no logo

An unknown symbol answers `404`, which shows as a broken image. With `fallback=monogram` it answers with a tile of the company's initials instead, and sets the `X-Logo-Fallback` header so you can tell:

```html
<img src="https://api.dharun.dev/v1/logos/NEWCO?fallback=monogram" alt="NewCo" width="32" height="32">
```

## A React component

```jsx
export function CompanyLogo({ symbol, size = 32 }) {
  const src = `https://api.dharun.dev/v1/logos/${encodeURIComponent(symbol)}?format=png&size=${size * 2}&fallback=monogram`;
  return <img src={src} alt={symbol} width={size} height={size} loading="lazy" style={{ borderRadius: 6 }} />;
}
```

## Details and search

`/v1/companies/{symbol}` returns the name, sector, industry, ISIN, market capitalisation, index memberships and the logo URLs:

```sh
curl https://api.dharun.dev/v1/companies/RELIANCE
curl "https://api.dharun.dev/v1/companies?q=tata&limit=5"
```

Logos are trademarks of their companies. Use them to identify the company, as a stock app does, not to suggest endorsement.
