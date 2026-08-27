package jd

import (
	"context"
	"fmt"
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
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	if err := chromedp.Run(browserCtx, mobileActions(), chromedp.Navigate("https://plogin.m.jd.com/login/login")); err != nil {
		return fmt.Errorf("open JD login: %w", err)
	}
	fmt.Println("请在打开的 Chrome 窗口中自行完成京东登录；本命令不会读取账号、密码、短信码或二维码。")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Minute)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return errorsTimeout("JD login was not completed within 10 minutes")
		case <-ticker.C:
			var location, body string
			if err := chromedp.Run(browserCtx, chromedp.Location(&location), chromedp.Text("body", &body, chromedp.ByQuery)); err != nil {
				continue
			}
			if containsVerification(body, location) {
				fmt.Println("检测到京东安全验证，请在浏览器中人工完成。")
				continue
			}
			if !containsLogin(body, location) && !strings.Contains(location, "plogin") {
				fmt.Println("京东登录状态已保存到独立 Chrome Profile。现在可以关闭本命令并启动服务。")
				return nil
			}
		}
	}
}

type loginTimeoutError string

func (e loginTimeoutError) Error() string { return string(e) }
func errorsTimeout(s string) error        { return loginTimeoutError(s) }
