package render

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

// Options configures headless rendering.
type Options struct {
	// Timeout bounds the whole render.
	Timeout time.Duration
	// Wait is how long to let scripts run after the DOM is ready.
	Wait time.Duration
	// UserAgent is sent by the headless browser. Empty keeps Chrome's own.
	UserAgent string
}

// Headless loads url in a local headless Chrome and returns the rendered
// HTML. Chrome or Chromium must be installed.
func Headless(ctx context.Context, url string, opts Options) ([]byte, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 45 * time.Second
	}
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", "new"))
	if opts.UserAgent != "" {
		allocOpts = append(allocOpts, chromedp.UserAgent(opts.UserAgent))
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	ctx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	ctx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	wrap := func(err error) error {
		return fmt.Errorf("headless render: %w (is Chrome installed?)", err)
	}
	for _, step := range []chromedp.Action[chromedp.Void]{
		chromedp.Navigate(url),
		chromedp.WaitReady(chromedp.CSS("body")),
		chromedp.Sleep(opts.Wait),
	} {
		if _, err := chromedp.Run(ctx, step); err != nil {
			return nil, wrap(err)
		}
	}
	out, err := chromedp.Run(ctx, chromedp.OuterHTML(chromedp.CSS("html")))
	if err != nil {
		return nil, wrap(err)
	}
	return []byte(out), nil
}

// Compare measures how much visible text only exists after JavaScript runs.
func Compare(raw, rendered []byte) *types.RenderResult {
	rawText := textdiff.VisibleText(raw)
	renderedText := textdiff.VisibleText(rendered)
	r := &types.RenderResult{
		RawTextLength:      len(rawText),
		RenderedTextLength: len(renderedText),
		Similarity:         textdiff.Similarity(rawText, renderedText),
	}
	if r.RenderedTextLength > r.RawTextLength && r.RenderedTextLength > 0 {
		r.JSOnlyShare = 1 - float64(r.RawTextLength)/float64(r.RenderedTextLength)
	}
	return r
}
