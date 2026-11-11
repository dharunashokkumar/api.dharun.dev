package festivals

import (
	"testing"
	"time"
)

// Dates of 2025 as the published almanacs and holiday lists give them.
func TestYear2025(t *testing.T) {
	started := time.Now()
	list, err := Year(2025)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("computed in %v", time.Since(started))
	got := map[string]string{}
	for _, f := range list {
		if _, seen := got[f.Name]; !seen {
			got[f.Name] = f.Date
		}
	}
	want := map[string]string{
		"Makar Sankranti / Pongal": "2025-01-14", "Vasant Panchami": "2025-02-02", "Thaipusam": "2025-02-11", "Maha Shivaratri": "2025-02-26",
		"Holika Dahan": "2025-03-13", "Holi": "2025-03-14", "Ugadi / Gudi Padwa": "2025-03-30", "Ram Navami": "2025-04-06",
		"Tamil New Year (Puthandu)": "2025-04-14", "Easter Sunday": "2025-04-20", "Good Friday": "2025-04-18", "Akshaya Tritiya": "2025-04-30",
		"Buddha Purnima": "2025-05-12", "Raksha Bandhan": "2025-08-09", "Krishna Janmashtami": "2025-08-16", "Ganesh Chaturthi": "2025-08-27",
		"Onam (Thiruvonam)": "2025-09-05", "Navaratri begins": "2025-09-22", "Dussehra / Vijayadashami": "2025-10-02", "Diwali (Lakshmi Puja)": "2025-10-20",
		"Guru Nanak Jayanti": "2025-11-05", "Karthigai Deepam": "2025-12-03", "Republic Day": "2025-01-26",
	}
	for name, date := range want {
		if got[name] != date {
			t.Errorf("%s = %s, want %s", name, got[name], date)
		}
	}
	for _, f := range list {
		if f.Kind == "islamic" {
			t.Logf("%s %s", f.Date, f.Name)
		}
	}
}

// 2026 has an extra month (adhika Jyeshtha), which pushes the later festivals out.
func TestYear2026(t *testing.T) {
	list, err := Year(2026)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range list {
		if _, seen := got[f.Name]; !seen {
			got[f.Name] = f.Date
		}
	}
	for name, date := range map[string]string{
		"Maha Shivaratri": "2026-02-15", "Holi": "2026-03-04", "Raksha Bandhan": "2026-08-28", "Krishna Janmashtami": "2026-09-04",
		"Ganesh Chaturthi": "2026-09-14", "Dussehra / Vijayadashami": "2026-10-20", "Diwali (Lakshmi Puja)": "2026-11-08", "Makar Sankranti / Pongal": "2026-01-14",
	} {
		if got[name] != date {
			t.Errorf("%s = %s, want %s", name, got[name], date)
		}
	}
}
