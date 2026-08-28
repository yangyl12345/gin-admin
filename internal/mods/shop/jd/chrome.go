package jd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/internal/mods/shop/schema"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

var (
	ErrLoginRequired    = errors.New("JD login is required")
	ErrCaptchaBlocked   = errors.New("JD verification or captcha is required")
	ErrNotSelfOperated  = errors.New("JD product is not confirmed as self-operated")
	ErrPriceUnavailable = errors.New("JD price is unavailable")

	skuPattern   = regexp.MustCompile(`(?i)(?:product/|item\.jd\.com/)([0-9]{5,})`)
	pricePattern = regexp.MustCompile(`(?i)(?:应付金额|实付款|到手价|京东价|售价|价格|[¥￥])[^0-9]{0,20}([0-9]+(?:\.[0-9]{1,2})?)`)
)

const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"

const (
	desktopUserAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	desktopSearchProbeURL = "https://search.jd.com/Search?keyword=%E6%89%8B%E6%9C%BA&enc=utf-8"
)

type ChromeClient struct {
	mu          sync.Mutex
	rootCtx     context.Context
	cancelRoot  context.CancelFunc
	cancelAlloc context.CancelFunc
}

func NewChromeClient() Client { return new(ChromeClient) }

// jdCDPErrorf suppresses only the protocol events that newer Chrome versions
// emit in a shape the pinned cdproto version cannot decode. These Network and
// Cookie events are not used by the JD workflow, so navigation and DOM actions
// can safely continue. All other browser errors remain visible.
func jdCDPErrorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if isBenignCDPCompatibilityError(message) {
		return
	}
	log.Printf(format, args...)
}

func isBenignCDPCompatibilityError(message string) bool {
	if !strings.Contains(message, "could not unmarshal event") {
		return false
	}
	if strings.Contains(message, "unknown IPAddressSpace value: Loopback") {
		return true
	}
	return strings.Contains(message, "parse error:") && strings.Contains(message, "cookiePart")
}

func (a *ChromeClient) ensureBrowser() (context.Context, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rootCtx != nil && a.rootCtx.Err() == nil {
		return a.rootCtx, nil
	}

	cfg := config.C.Shop.JD
	profile, err := filepath.Abs(cfg.UserDataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve JD profile path: %w", err)
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.ExecPath(cfg.ChromeExecutable),
		chromedp.UserDataDir(profile),
		chromedp.UserAgent(mobileUserAgent),
		chromedp.WindowSize(390, 844),
		chromedp.Flag("lang", "zh-CN"),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-extensions", true),
	)
	if cfg.Headless {
		opts = append(opts, chromedp.Headless)
	} else {
		opts = append(opts, chromedp.Flag("headless", false))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	rootCtx, cancelRoot := chromedp.NewContext(allocCtx, chromedp.WithErrorf(jdCDPErrorf))
	if err := chromedp.Run(rootCtx); err != nil {
		cancelRoot()
		cancelAlloc()
		return nil, fmt.Errorf("start JD Chrome: %w", err)
	}
	a.rootCtx = rootCtx
	a.cancelRoot = cancelRoot
	a.cancelAlloc = cancelAlloc
	return rootCtx, nil
}

func (a *ChromeClient) tab(ctx context.Context) (context.Context, context.CancelFunc, error) {
	root, err := a.ensureBrowser()
	if err != nil {
		return nil, nil, err
	}
	tabCtx, cancelTab := chromedp.NewContext(root)
	timeout := config.C.Shop.JD.NavigationTimeout
	if timeout <= 0 {
		timeout = 45
	}
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, time.Duration(timeout)*time.Second)
	cancel := func() {
		cancelTimeout()
		cancelTab()
	}
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-tabCtx.Done():
		}
	}()
	return tabCtx, cancel, nil
}

func mobileActions() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := emulation.SetDeviceMetricsOverride(390, 844, 3, true).Do(ctx); err != nil {
			return err
		}
		if err := emulation.SetTouchEmulationEnabled(true).Do(ctx); err != nil {
			return err
		}
		if err := emulation.SetLocaleOverride().WithLocale("zh_CN").Do(ctx); err != nil {
			return err
		}
		return emulation.SetTimezoneOverride("Asia/Shanghai").Do(ctx)
	})
}

func desktopActions() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := emulation.SetDeviceMetricsOverride(1280, 900, 1, false).Do(ctx); err != nil {
			return err
		}
		if err := emulation.SetTouchEmulationEnabled(false).Do(ctx); err != nil {
			return err
		}
		if err := emulation.SetUserAgentOverride(desktopUserAgent).Do(ctx); err != nil {
			return err
		}
		if err := emulation.SetLocaleOverride().WithLocale("zh_CN").Do(ctx); err != nil {
			return err
		}
		return emulation.SetTimezoneOverride("Asia/Shanghai").Do(ctx)
	})
}

func categoryBrowserActions(raw string) chromedp.Action {
	u, err := url.Parse(raw)
	if err == nil {
		switch strings.ToLower(u.Hostname()) {
		case "list.jd.com", "search.jd.com", "channel.jd.com":
			return desktopActions()
		}
	}
	return mobileActions()
}

func (a *ChromeClient) SessionStatus(ctx context.Context) (*schema.SessionStatus, error) {
	now := time.Now()
	tabCtx, cancel, err := a.tab(ctx)
	if err != nil {
		return &schema.SessionStatus{LastCheckedAt: now, ErrorSummary: err.Error()}, err
	}
	defer cancel()
	var location, body string
	err = chromedp.Run(tabCtx,
		mobileActions(),
		chromedp.Navigate("https://home.m.jd.com/myJd/newhome.action"),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Location(&location),
		chromedp.Text("body", &body, chromedp.ByQuery),
	)
	status := &schema.SessionStatus{LastCheckedAt: now}
	if err != nil {
		status.ErrorSummary = err.Error()
		return status, err
	}
	if err := schema.ValidateJDNavigationURL(location); err != nil {
		status.ErrorSummary = err.Error()
		return status, err
	}
	status.CaptchaBlocked = containsVerification(body, location)
	status.Authenticated = !status.CaptchaBlocked && !containsLogin(body, location)
	if status.CaptchaBlocked {
		status.ErrorSummary = ErrCaptchaBlocked.Error()
	} else if !status.Authenticated {
		status.ErrorSummary = ErrLoginRequired.Error()
	}
	return status, nil
}

type discoveredDOMItem struct {
	Href    string `json:"href"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Context string `json:"context"`
}

func (a *ChromeClient) DiscoverCategory(ctx context.Context, category *schema.JDCategory) ([]schema.DiscoveredProduct, error) {
	if err := schema.ValidateJDCategoryURL(category.SourceURL); err != nil {
		return nil, err
	}
	seen := make(map[string]schema.DiscoveredProduct)
	for page := 1; page <= category.MaxPages; page++ {
		pageURL, err := withPage(category.SourceURL, page)
		if err != nil {
			return nil, err
		}
		tabCtx, cancel, err := a.tab(ctx)
		if err != nil {
			return nil, err
		}
		var location string
		var items []discoveredDOMItem
		err = chromedp.Run(tabCtx,
			categoryBrowserActions(pageURL),
			chromedp.Navigate(pageURL),
			chromedp.WaitReady("body", chromedp.ByQuery),
		)
		if err != nil {
			cancel()
			return nil, err
		}
		_ = chromedp.Run(tabCtx, chromedp.Poll(`(() => {
  const body = document.body && document.body.innerText || '';
  const hasProduct = Array.from(document.querySelectorAll('a[href]')).some(a => /item\.jd\.com|item\.m\.jd\.com|\/product\/[0-9]+/.test(a.href || ''));
  return hasProduct || /login|passport|captcha|verify/i.test(location.href) || /安全验证|滑动验证|验证码|登录京东|请先登录/.test(body);
})()`, nil, chromedp.WithPollingInterval(500*time.Millisecond), chromedp.WithPollingTimeout(15*time.Second)))
		var body string
		err = chromedp.Run(tabCtx,
			chromedp.Location(&location),
			chromedp.Text("body", &body, chromedp.ByQuery),
			chromedp.Evaluate(`(() => Array.from(document.querySelectorAll('a[href]')).map(a => {
  const href = a.href || '';
  const box = a.closest('li,[class*="item"],[class*="product"],[class*="goods"]') || a.parentElement;
  const img = a.querySelector('img') || (box && box.querySelector('img'));
  return {href, name: (a.innerText || (img && img.alt) || '').trim(), image: img ? (img.currentSrc || img.src || '') : '', context: box ? (box.innerText || '').slice(0, 1000) : ''};
}).filter(x => /item\.jd\.com|item\.m\.jd\.com|\/product\/[0-9]+/.test(x.href)))()`, &items),
		)
		cancel()
		if err != nil {
			return nil, err
		}
		if err := schema.ValidateJDNavigationURL(location); err != nil {
			return nil, err
		}
		if containsVerification(body, location) {
			return nil, ErrCaptchaBlocked
		}
		if containsLogin(body, location) {
			return nil, ErrLoginRequired
		}
		if len(items) == 0 {
			if page == 1 {
				return nil, fmt.Errorf("%w: no JD product links found at %s", ErrPriceUnavailable, location)
			}
			break
		}
		for _, item := range items {
			sku := parseSKU(item.Href)
			if sku == "" || !containsSelfOperated(item.Context) {
				continue
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = "JD SKU " + sku
			}
			seen[sku] = schema.DiscoveredProduct{
				SKU: sku, Name: name, CanonicalURL: canonicalURL(sku), ImageURL: item.Image, SelfOperated: true,
			}
		}
	}
	result := make([]schema.DiscoveredProduct, 0, len(seen))
	for _, item := range seen {
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%w: no JD self-operated products found", ErrNotSelfOperated)
	}
	return result, nil
}

type priceDOMResult struct {
	Price string `json:"price"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (a *ChromeClient) FetchPublicPrice(ctx context.Context, product *schema.JDProduct) (*schema.PriceObservation, error) {
	tabCtx, cancel, err := a.tab(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var location string
	var result priceDOMResult
	err = chromedp.Run(tabCtx,
		mobileActions(),
		chromedp.Navigate(product.CanonicalURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(time.Second),
		chromedp.Location(&location),
		chromedp.Evaluate(priceExtractionScript(false), &result),
	)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateJDNavigationURL(location); err != nil {
		return nil, err
	}
	if containsVerification(result.Body, location) {
		return nil, ErrCaptchaBlocked
	}
	if !containsSelfOperated(result.Body) {
		return nil, ErrNotSelfOperated
	}
	fen, err := ParseYuanToFen(result.Price)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(result.Title)
	if name == "" {
		name = product.Name
	}
	return &schema.PriceObservation{SKU: product.SKU, Name: name, CanonicalURL: product.CanonicalURL, PriceFen: fen, Currency: "CNY", SelfOperated: true}, nil
}

func (a *ChromeClient) FetchCheckoutPreview(ctx context.Context, product *schema.JDProduct) (*schema.PriceObservation, error) {
	tabCtx, cancel, err := a.tab(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()
	var location, body string
	err = chromedp.Run(tabCtx,
		mobileActions(),
		chromedp.Navigate(product.CanonicalURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Text("body", &body, chromedp.ByQuery),
	)
	if err != nil {
		return nil, err
	}
	if containsVerification(body, product.CanonicalURL) {
		return nil, ErrCaptchaBlocked
	}
	if containsLogin(body, product.CanonicalURL) {
		return nil, ErrLoginRequired
	}
	if !containsSelfOperated(body) {
		return nil, ErrNotSelfOperated
	}

	var clicked bool
	err = chromedp.Run(tabCtx,
		chromedp.Evaluate(`(() => {
  const nodes = Array.from(document.querySelectorAll('a,button,[role="button"]'));
  const target = nodes.find(n => (n.innerText || '').trim().includes('立即购买'));
  if (!target) return false;
  target.click();
  return true;
})()`, &clicked),
	)
	if err != nil {
		return nil, err
	}
	if !clicked {
		return nil, fmt.Errorf("%w: buy-now control not found", ErrPriceUnavailable)
	}

	var result priceDOMResult
	err = chromedp.Run(tabCtx,
		chromedp.Sleep(3*time.Second),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Location(&location),
		chromedp.Evaluate(priceExtractionScript(true), &result),
	)
	if err != nil {
		return nil, err
	}
	if err := schema.ValidateJDNavigationURL(location); err != nil {
		return nil, err
	}
	if containsVerification(result.Body, location) {
		return nil, ErrCaptchaBlocked
	}
	if containsLogin(result.Body, location) {
		return nil, ErrLoginRequired
	}
	if !containsSelfOperated(result.Body) {
		return nil, ErrNotSelfOperated
	}
	if strings.Contains(location, "/product/") || (!strings.Contains(result.Body, "订单确认") && !strings.Contains(result.Body, "提交订单") && !strings.Contains(result.Body, "应付金额")) {
		return nil, fmt.Errorf("%w: checkout confirmation page was not reached", ErrPriceUnavailable)
	}
	if strings.Contains(result.Body, "提交订单成功") || strings.Contains(result.Body, "支付订单") {
		return nil, errors.New("unsafe JD navigation reached an order result page")
	}
	fen, err := ParseYuanToFen(result.Price)
	if err != nil {
		return nil, err
	}
	return &schema.PriceObservation{SKU: product.SKU, Name: product.Name, CanonicalURL: product.CanonicalURL, PriceFen: fen, Currency: "CNY", SelfOperated: true}, nil
}

func (a *ChromeClient) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancelRoot != nil {
		a.cancelRoot()
	}
	if a.cancelAlloc != nil {
		a.cancelAlloc()
	}
	a.rootCtx = nil
	a.cancelRoot = nil
	a.cancelAlloc = nil
	return nil
}

func priceExtractionScript(checkout bool) string {
	selectors := `['[data-price]','meta[itemprop="price"]','[class*="price"]','[class*="amount"]']`
	keywords := "京东价|到手价|售价|价格"
	if checkout {
		selectors = `['[class*="pay"] [class*="price"]','[class*="total"] [class*="price"]','[class*="amount"]','[data-price]']`
		keywords = "应付金额|实付款|总计|合计"
	}
	preferKeyword := "false"
	if checkout {
		preferKeyword = "true"
	}
	return fmt.Sprintf(`(() => {
  const body = (document.body && document.body.innerText || '').slice(0, 200000);
  let price = '';
  if (%s) {
    const match = body.match(/(?:%s)[^0-9]{0,20}([0-9]+(?:\.[0-9]{1,2})?)/);
    if (match) price = match[1];
  }
  for (const selector of %s) {
    if (price) break;
    const node = document.querySelector(selector);
    if (!node) continue;
    const value = node.getAttribute('content') || node.getAttribute('data-price') || node.innerText || '';
    const match = String(value).match(/[0-9]+(?:\.[0-9]{1,2})?/);
    if (match && Number(match[0]) > 0) { price = match[0]; break; }
  }
  if (!price) {
    const match = body.match(/(?:%s)[^0-9]{0,20}([0-9]+(?:\.[0-9]{1,2})?)/);
    if (match) price = match[1];
  }
  return {price, title: document.title || '', body};
})()`, preferKeyword, keywords, selectors, keywords)
}

func ParseYuanToFen(raw string) (int64, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(raw, "¥", ""), "￥", ""))
	if raw == "" {
		return 0, ErrPriceUnavailable
	}
	if m := pricePattern.FindStringSubmatch(raw); len(m) > 1 {
		raw = m[1]
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, ErrPriceUnavailable
	}
	fen := int64(math.Round(v * 100))
	if fen <= 0 {
		return 0, ErrPriceUnavailable
	}
	return fen, nil
}

func parseSKU(raw string) string {
	m := skuPattern.FindStringSubmatch(raw)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func canonicalURL(sku string) string { return "https://item.m.jd.com/product/" + sku + ".html" }

func containsSelfOperated(body string) bool {
	return strings.Contains(body, "京东自营") || strings.Contains(body, "自营")
}

func containsLogin(body, location string) bool {
	v := strings.ToLower(location)
	return strings.Contains(v, "login") || strings.Contains(v, "passport") || strings.Contains(body, "登录京东") || strings.Contains(body, "请先登录")
}

func containsVerification(body, location string) bool {
	v := strings.ToLower(location)
	return strings.Contains(v, "captcha") || strings.Contains(v, "verify") || strings.Contains(body, "安全验证") || strings.Contains(body, "滑动验证") || strings.Contains(body, "验证码")
}

func withPage(raw string, page int) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
