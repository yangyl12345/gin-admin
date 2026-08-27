package schema

import "testing"

func TestValidateJDCategoryURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "list", value: "https://list.jd.com/list.html?cat=1,2,3"},
		{name: "search", value: "https://search.jd.com/Search?keyword=phone"},
		{name: "mobile category", value: "https://m.jd.com/category/all.html"},
		{name: "mobile search", value: "https://so.m.jd.com/ware/search.action?keyword=phone"},
		{name: "http rejected", value: "http://list.jd.com/list.html", wantErr: true},
		{name: "custom port rejected", value: "https://list.jd.com:8443/list.html", wantErr: true},
		{name: "lookalike rejected", value: "https://list.jd.com.example.com/list.html", wantErr: true},
		{name: "unrecognized subdomain rejected", value: "https://evil.jd.com/list.html", wantErr: true},
		{name: "non category path rejected", value: "https://m.jd.com/account/profile", wantErr: true},
		{name: "host root rejected", value: "https://m.jd.com/", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJDCategoryURL(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateJDCategoryURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShopSettingFormValidate(t *testing.T) {
	if err := (&ShopSettingForm{CandidateDropPercent: 15, AlertDropPercent: 20, RecoveryDropPercent: 15}).Validate(); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	if err := (&ShopSettingForm{CandidateDropPercent: 25, AlertDropPercent: 20, RecoveryDropPercent: 15}).Validate(); err == nil {
		t.Fatal("candidate threshold greater than alert threshold should fail")
	}
	if err := (&ShopSettingForm{CandidateDropPercent: 15, AlertDropPercent: 20, RecoveryDropPercent: 20}).Validate(); err == nil {
		t.Fatal("recovery threshold equal to alert threshold should fail")
	}
}

func TestFormatPriceYuan(t *testing.T) {
	if got := FormatPriceYuan(12345); got != "123.45" {
		t.Fatalf("FormatPriceYuan() = %s", got)
	}
}
