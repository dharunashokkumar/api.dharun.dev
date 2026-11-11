package payqr

import (
	"fmt"
	"testing"
)

func field(id, value string) string { return fmt.Sprintf("%s%02d%s", id, len(value), value) }

func emv(body string) string {
	s := body + "6304"
	return s + fmt.Sprintf("%04X", CRC16(s))
}

func TestCRC(t *testing.T) {
	// CRC-16/CCITT-FALSE check value.
	if got := CRC16("123456789"); got != 0x29B1 {
		t.Fatalf("crc %04X", got)
	}
}

func TestUPILink(t *testing.T) {
	r, err := Parse("upi://pay?pa=shop@okaxis&pn=Sri%20Murugan%20Stores&am=250.00&cu=INR&mc=5411&tn=Rice")
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != "upi" || r.Payee != "shop@okaxis" || r.PayeeName != "Sri Murugan Stores" || r.Amount != "250.00" || r.MerchantKind != "grocery stores and supermarkets" || len(r.Warnings) != 0 {
		t.Errorf("%+v", r)
	}
	r, _ = Parse("upi://pay?pn=Nobody&am=abc")
	if len(r.Warnings) != 2 {
		t.Errorf("warnings %v", r.Warnings)
	}
}

func TestEMVCo(t *testing.T) {
	upi := field("00", "A000000677010111") + field("01", "shop@okaxis")
	body := field("00", "01") + field("01", "12") + field("04", "5123450000000000") + field("26", upi) + field("52", "5812") + field("53", "356") +
		field("54", "99.50") + field("58", "IN") + field("59", "Annapoorna") + field("60", "Coimbatore") + field("61", "641001") + field("62", field("05", "ORDER42"))
	r, err := Parse(emv(body))
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != "emvco" || r.Payee != "shop@okaxis" || r.Currency != "INR" || r.Amount != "99.50" || r.City != "Coimbatore" || r.Reference != "ORDER42" ||
		r.CRCValid == nil || !*r.CRCValid || r.Dynamic == nil || !*r.Dynamic || len(r.Networks) != 2 || r.Networks[1] != "UPI" {
		t.Errorf("%+v", r)
	}
	broken := emv(body)
	broken = broken[:len(broken)-1] + "0"
	if r, _ := Parse(broken); r.CRCValid == nil || *r.CRCValid {
		t.Error("a changed CRC passed")
	}
	for _, bad := range []string{"", "hello", "000201" + "59ZZ"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
