//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/playwright-community/playwright-go"
)

var (
	pw      *playwright.Playwright
	browser playwright.Browser
)

func TestMain(m *testing.M) {
	var err error
	pw, err = playwright.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "playwright.Run: %v\n", err)
		os.Exit(1)
	}
	browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args:     []string{"--no-sandbox", "--disable-gpu"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "browser launch: %v\n", err)
		pw.Stop()
		os.Exit(1)
	}

	code := m.Run()

	browser.Close()
	pw.Stop()
	os.Exit(code)
}

type Harness struct {
	t      *testing.T
	server *httptest.Server
	Page   playwright.Page
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("cannot find repo root (go.mod)")
		}
		dir = parent
	}
}

func frontendDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "frontend")
}

func testdataDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "examples")
}

func goldenDir(t *testing.T) string {
	t.Helper()
	return testdataDir(t)
}

// NewHarness creates a Harness with the given viewport dimensions.
func NewHarness(t *testing.T, cfg *config.NotificationConfig, width, height int) *Harness {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(os.DirFS(frontendDir(t))))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{Width: width, Height: height},
	})
	if err != nil {
		t.Fatalf("browser context: %v", err)
	}
	t.Cleanup(func() { ctx.Close() })

	page, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	h := &Harness{t: t, server: srv, Page: page}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := h.injectMockBindings(cfgJSON); err != nil {
		t.Fatalf("inject mocks: %v", err)
	}

	resp, err := page.Goto(srv.URL+"/index.html", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("goto: %v", err)
	}
	if resp.Status() != 200 {
		t.Fatalf("page status: %d", resp.Status())
	}
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		t.Fatalf("wait for load: %v", err)
	}

	return h
}

// Start creates a Harness with the standard notification viewport (380x220).
func Start(t *testing.T, cfg *config.NotificationConfig) *Harness {
	t.Helper()
	return NewHarness(t, cfg, 380, 220)
}

// StartTall creates a Harness with the taller viewport for image carousels (380x480).
func StartTall(t *testing.T, cfg *config.NotificationConfig) *Harness {
	t.Helper()
	return NewHarness(t, cfg, 380, 480)
}

func (h *Harness) injectMockBindings(cfgJSON []byte) error {
	script := fmt.Sprintf(`
window.__e2e = { respondCalls: [], readyCalled: false, openHelpCalled: false };
window.go = {
  "github.com/TsekNet/hermes/internal/app": {
    App: {
      GetConfig: function() {
        return Promise.resolve(%s);
      },
      Respond: function(value) {
        window.__e2e.respondCalls.push(value);
      },
      Ready: function() {
        window.__e2e.readyCalled = true;
      },
      DeferralAllowed: function() {
        return Promise.resolve(true);
      },
      OpenHelp: function() {
        window.__e2e.openHelpCalled = true;
      }
    }
  }
};
`, string(cfgJSON))

	return h.Page.AddInitScript(playwright.Script{Content: playwright.String(script)})
}

func (h *Harness) RespondCalls() []string {
	h.t.Helper()
	val, err := h.Page.Evaluate(`window.__e2e.respondCalls`)
	if err != nil {
		h.t.Fatalf("read respondCalls: %v", err)
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, len(arr))
	for i, v := range arr {
		result[i] = fmt.Sprintf("%v", v)
	}
	return result
}

func (h *Harness) ReadyCalled() bool {
	h.t.Helper()
	val, err := h.Page.Evaluate(`window.__e2e.readyCalled`)
	if err != nil {
		h.t.Fatalf("read readyCalled: %v", err)
	}
	b, _ := val.(bool)
	return b
}

func (h *Harness) OpenHelpCalled() bool {
	h.t.Helper()
	val, err := h.Page.Evaluate(`window.__e2e.openHelpCalled`)
	if err != nil {
		h.t.Fatalf("read openHelpCalled: %v", err)
	}
	b, _ := val.(bool)
	return b
}

func (h *Harness) TextContent(selector string) string {
	h.t.Helper()
	text, err := h.Page.Locator(selector).TextContent()
	if err != nil {
		h.t.Fatalf("text content %q: %v", selector, err)
	}
	return strings.TrimSpace(text)
}

func (h *Harness) IsVisible(selector string) bool {
	h.t.Helper()
	visible, err := h.Page.Locator(selector).IsVisible()
	if err != nil {
		return false
	}
	return visible
}

func (h *Harness) ButtonCount() int {
	h.t.Helper()
	count, err := h.Page.Locator("#buttons .btn").Count()
	if err != nil {
		h.t.Fatalf("button count: %v", err)
	}
	return count
}

func (h *Harness) ButtonLabels() []string {
	h.t.Helper()
	locs, err := h.Page.Locator("#buttons .btn").All()
	if err != nil {
		h.t.Fatalf("button all: %v", err)
	}
	labels := make([]string, len(locs))
	for i, loc := range locs {
		text, err := loc.TextContent()
		if err != nil {
			h.t.Fatalf("button %d text: %v", i, err)
		}
		labels[i] = strings.TrimSpace(text)
	}
	return labels
}

func (h *Harness) Screenshot() []byte {
	h.t.Helper()
	data, err := h.Page.Screenshot(playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(true),
	})
	if err != nil {
		h.t.Fatalf("screenshot: %v", err)
	}
	return data
}

func LoadConfig(t *testing.T, name string) *config.NotificationConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(t), name))
	if err != nil {
		t.Fatalf("read config %s: %v", name, err)
	}
	var cfg config.NotificationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config %s: %v", name, err)
	}
	cfg.ApplyDefaults()
	return &cfg
}

func AllTestdataConfigs(t *testing.T) []string {
	t.Helper()
	entries, err := fs.Glob(os.DirFS(testdataDir(t)), "*.json")
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	return entries
}

func UpdateGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	dir := goldenDir(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir golden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write golden %s: %v", name, err)
	}
}

func ReadGolden(t *testing.T, name string) ([]byte, bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(goldenDir(t), name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		t.Fatalf("read golden %s: %v", name, err)
	}
	return data, true
}
