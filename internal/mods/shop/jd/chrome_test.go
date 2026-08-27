package jd

import "testing"

func TestParseYuanToFen(t *testing.T) {
	tests := []struct {
		value   string
		want    int64
		wantErr bool
	}{
		{"123.45", 12345, false},
		{"￥9.9", 990, false},
		{"到手价 88", 8800, false},
		{"0", 0, true},
		{"not-a-price", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseYuanToFen(tt.value)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Fatalf("ParseYuanToFen(%q) = %d, %v; want %d, wantErr=%v", tt.value, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestParseSKU(t *testing.T) {
	if got := parseSKU("https://item.m.jd.com/product/123456789.html"); got != "123456789" {
		t.Fatalf("parseSKU() = %s", got)
	}
	if got := parseSKU("https://item.jd.com/987654.html"); got != "987654" {
		t.Fatalf("parseSKU() = %s", got)
	}
}
