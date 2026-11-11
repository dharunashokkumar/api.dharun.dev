// Command einvoiceqr prints the text of the sample e-invoice QR code: a JSON Web Token
// shaped like the signed QR the Invoice Registration Portal issues, for the documented
// invoice example. It is not signed: its signature is the word "sample". MIT-0.
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func main() {
	encode := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	irn := sha256.Sum256([]byte("33AABCS1429B1Z1|2026-27|INV|INV-2026-0042"))
	data, _ := json.Marshal(map[string]any{
		"SellerGstin": "33AABCS1429B1Z1", "BuyerGstin": "29AAACK6812C1ZS", "DocNo": "INV-2026-0042", "DocTyp": "INV",
		"DocDt": "14/10/2026", "TotInvVal": 13013, "ItemCnt": 2, "MainHsnCode": "1006",
		"Irn": hex.EncodeToString(irn[:]), "IrnDt": "2026-10-14 11:05:00",
	})
	header := encode(map[string]string{"alg": "RS256", "kid": "SAMPLE", "typ": "JWT"})
	payload := encode(map[string]string{"data": string(data), "iss": "Sample, not issued by NIC"})
	fmt.Print(header + "." + payload + "." + base64.RawURLEncoding.EncodeToString([]byte("sample")))
}
