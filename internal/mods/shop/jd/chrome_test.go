package jd

import (
	"net/url"
	"testing"
)

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
	if got := parseSKU("https://chat.jd.com/index.action?entry=jd_search&pid=100278222022"); got != "100278222022" {
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

func TestDesktopUserAgentForProduct(t *testing.T) {
	got, err := desktopUserAgentForProduct("Chrome/152.0.7977.64")
	if err != nil {
		t.Fatal(err)
	}
	want := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
	if got != want {
		t.Fatalf("desktopUserAgentForProduct() = %q; want %q", got, want)
	}
	if _, err := desktopUserAgentForProduct("Firefox/150.0"); err == nil {
		t.Fatal("desktopUserAgentForProduct() accepted an unsupported browser product")
	}
}

func TestWithPage(t *testing.T) {
	original := "https://search.jd.com/Search?keyword=phone&enc=utf-8"
	preserved, err := withPage(original, 1)
	if err != nil {
		t.Fatal(err)
	}
	if preserved != original {
		t.Fatalf("first search page URL was rewritten: got %s, want %s", preserved, original)
	}

	first, err := withPage("https://search.jd.com/Search?keyword=phone&enc=utf-8&page=1&s=61", 1)
	if err != nil {
		t.Fatal(err)
	}
	firstURL, err := url.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	if firstURL.Query().Has("page") || firstURL.Query().Has("s") {
		t.Fatalf("first search page retained pagination parameters: %s", first)
	}

	second, err := withPage("https://search.jd.com/Search?keyword=phone&enc=utf-8", 2)
	if err != nil {
		t.Fatal(err)
	}
	secondURL, err := url.Parse(second)
	if err != nil {
		t.Fatal(err)
	}
	if secondURL.Query().Get("page") != "3" || secondURL.Query().Get("s") != "61" {
		t.Fatalf("second search page parameters are incorrect: %s", second)
	}

	listPage, err := withPage("https://list.jd.com/list.html?cat=9987,653,655", 2)
	if err != nil {
		t.Fatal(err)
	}
	listURL, err := url.Parse(listPage)
	if err != nil {
		t.Fatal(err)
	}
	if listURL.Query().Get("page") != "2" {
		t.Fatalf("list page parameter is incorrect: %s", listPage)
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
