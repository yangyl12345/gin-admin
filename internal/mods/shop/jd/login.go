package jd

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/chromedp/chromedp"
)

// RunLogin opens a visible, isolated Chrome profile and waits for the user to
// complete JD authentication. It never accepts account credentials itself.
func RunLogin(ctx context.Context) error {
	cfg := config.C.Shop.JD
	profile, err := filepath.Abs(cfg.UserDataDir)
	if err != nil {
		return err
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.ExecPath(cfg.ChromeExecutable),
		chromedp.UserDataDir(profile),
		chromedp.UserAgent(mobileUserAgent),
		chromedp.WindowSize(390, 844),
		chromedp.Flag("headless", false),
		chromedp.Flag("lang", "zh-CN"),
		chromedp.Flag("disable-sync", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithErrorf(jdCDPErrorf))
	defer cancelBrowser()

	if err := chromedp.Run(browserCtx, mobileActions(), chromedp.Navigate("https://plogin.m.jd.com/login/login")); err != nil {
		return fmt.Errorf("open JD login: %w", err)
	}
	fmt.Println("请在打开的 Chrome 窗口中自行完成京东登录；本命令不会读取账号、密码、短信码或二维码。")
	if err := waitForLoginPhase(browserCtx, func(location, body string) bool {
		return !containsVerification(body, location) && !containsLogin(body, location) && !strings.Contains(strings.ToLower(location), "plogin")
	}, "JD mobile login was not completed within 10 minutes"); err != nil {
		return err
	}
	fmt.Println("京东移动端登录已确认，正在验证桌面搜索会话。")

	if err := chromedp.Run(browserCtx, desktopActions(), chromedp.Navigate(desktopSearchProbeURL)); err != nil {
		return fmt.Errorf("open JD desktop search login: %w", err)
	}
	fmt.Println("如果页面显示京东桌面登录、网络错误或安全验证，请在当前 Chrome 窗口中人工完成；实际加载出京东自营商品后才算成功。")
	if err := waitForLoginPhase(browserCtx, func(location, body string) bool {
		return !containsVerification(body, location) && !containsLogin(body, location) && isDesktopSearchLocation(location) && containsSelfOperated(body)
	}, "JD desktop search login was not completed within 10 minutes"); err != nil {
		return err
	}
	fmt.Println("京东移动端和桌面搜索登录状态均已保存到独立 Chrome Profile。现在可以关闭本命令并启动服务。")
	return nil
}

func waitForLoginPhase(browserCtx context.Context, completed func(location, body string) bool, timeoutMessage string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Minute)
	defer timeout.Stop()
	verificationReported := false
	for {
		select {
		case <-browserCtx.Done():
			return browserCtx.Err()
		case <-timeout.C:
			return errorsTimeout(timeoutMessage)
		case <-ticker.C:
			var location, body string
			if err := chromedp.Run(browserCtx, chromedp.Location(&location), chromedp.Text("body", &body, chromedp.ByQuery)); err != nil {
				continue
			}
			if containsVerification(body, location) {
				if !verificationReported {
					fmt.Println("检测到京东安全验证，请在浏览器中人工完成。")
					verificationReported = true
				}
				continue
			}
			verificationReported = false
			if completed(location, body) {
				return nil
			}
		}
	}
}

func isDesktopSearchLocation(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && strings.EqualFold(u.Hostname(), "search.jd.com")
}

type loginTimeoutError string

func (e loginTimeoutError) Error() string { return string(e) }
func errorsTimeout(s string) error        { return loginTimeoutError(s) }
