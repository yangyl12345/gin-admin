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

func TestContainsLogin(t *testing.T) {
	if !containsLogin("个人用户登录", "https://passport.jd.com/new/login.aspx") {
		t.Fatal("desktop JD login page was not detected")
	}
	if containsLogin("手机商品列表", "https://search.jd.com/Search?keyword=phone") {
		t.Fatal("search result was detected as a login page")
	}
}

func TestDesktopSearchLocation(t *testing.T) {
	if !isDesktopSearchLocation("https://search.jd.com/Search?keyword=phone") {
		t.Fatal("desktop JD search URL was not recognized")
	}
	if isDesktopSearchLocation("https://passport.jd.com/new/login.aspx") {
		t.Fatal("desktop JD login URL was recognized as search")
	}
}

func TestIsBenignCDPCompatibilityError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "Chrome loopback address space",
			message: "could not unmarshal event: unknown IPAddressSpace value: Loopback",
			want:    true,
		},
		{
			name:    "Chrome cookie part parse error",
			message: "could not unmarshal event: parse error: expected string near offset 653 of 'cookiePart...'",
			want:    true,
		},
		{
			name:    "ordinary CDP error",
			message: "could not navigate to the requested page",
			want:    false,
		},
		{
			name:    "unrelated event decode error",
			message: "could not unmarshal event: unknown SecurityState value: FutureValue",
			want:    false,
		},
		{
			name:    "cookie parse text outside event decoder",
			message: "parse error: expected string near cookiePart",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBenignCDPCompatibilityError(tt.message); got != tt.want {
				t.Fatalf("isBenignCDPCompatibilityError(%q) = %v; want %v", tt.message, got, tt.want)
			}
		})
	}
}
