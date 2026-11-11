package api

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"strings"
	"testing"
)

func TestBatchCSV(t *testing.T) {
	body := "name,pin\nRavi,637001\nAsha,600001\nNobody,000000\n"
	got := request(t, http.MethodPost, "/v1/batch?kind=pincode&column=pin", body, "Content-Type", "text/csv")
	if got.status != http.StatusOK || !strings.HasPrefix(got.header.Get("Content-Type"), "text/csv") {
		t.Fatalf("csv batch = %d %s: %s", got.status, got.header.Get("Content-Type"), got.body)
	}
	rows, err := csv.NewReader(bytes.NewReader(got.body)).ReadAll()
	if err != nil || len(rows) != 4 {
		t.Fatalf("rows %v %v", rows, err)
	}
	head := strings.Join(rows[0], ",")
	if !strings.HasPrefix(head, "name,pin,pincode.status") || !strings.Contains(head, "pincode.district") {
		t.Errorf("header %s", head)
	}
	district := -1
	for i, h := range rows[0] {
		if h == "pincode.district" {
			district = i
		}
	}
	if rows[1][district] != "Namakkal" || rows[1][2] != "200" || rows[3][2] == "200" {
		t.Errorf("rows %v", rows)
	}
	if got := request(t, http.MethodPost, "/v1/batch?kind=nothing", body, "Content-Type", "text/csv"); got.status != http.StatusBadRequest {
		t.Errorf("unknown kind = %d", got.status)
	}
	if got := request(t, http.MethodPost, "/v1/batch?kind=pincode&column=missing", body, "Content-Type", "text/csv"); got.status != http.StatusBadRequest {
		t.Errorf("missing column = %d", got.status)
	}
}
