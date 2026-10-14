---
title: Write rupee amounts in words for invoices and cheques
description: Spell an amount the Indian way, with lakh and crore, rupees and paise, in English, Hindi or Tamil.
apis: words
published: 2026-10-14
---

Invoices, receipts and cheques print the amount in words, and in India that means lakh and crore, not hundred thousand and million.

## The request

```sh
curl "https://api.dharun.dev/v1/words/1,25,000.50?style=currency"
```

```json
{
  "data": {
    "input": "1,25,000.50",
    "number": "125000.50",
    "language": "en",
    "style": "currency",
    "words": "one lakh twenty-five thousand rupees and fifty paise only"
  }
}
```

Commas, spaces and underscores in the number are ignored, so `125000.50` and `1,25,000.50` give the same answer. Without `style=currency` you get the plain number: "one lakh twenty-five thousand point five zero".

## Hindi and Tamil

```sh
curl "https://api.dharun.dev/v1/words/1,25,000.50?style=currency&lang=hi"
# एक लाख पच्चीस हज़ार रुपये और पचास पैसे मात्र

curl "https://api.dharun.dev/v1/words/250?style=currency&lang=ta"
```

## On an invoice

```js
async function amountInWords(amount, lang = "en") {
  const response = await fetch(
    `https://api.dharun.dev/v1/words/${amount.toFixed(2)}?style=currency&lang=${lang}`
  );
  const body = await response.json();
  if (!response.ok) throw new Error(body.error.message);
  return body.data.words;
}

const words = await amountInWords(125000.5);
// "one lakh twenty-five thousand rupees and fifty paise only"
document.querySelector("#total-in-words").textContent =
  words.charAt(0).toUpperCase() + words.slice(1);
```

Pass the amount with two decimals (`toFixed(2)`), so that 0.5 rupees is read as fifty paise.
