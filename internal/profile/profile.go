package profile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"i2tor/internal/util"
)

type Options struct {
	AllowLocalhostAccess bool
	ProfileDisplayName   string
	HomePagePath         string
}

const fallbackStartPageLogoDataURI = "data:image/svg+xml;utf8,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 256 256'%3E%3Cdefs%3E%3ClinearGradient id='g' x1='0' y1='0' x2='1' y2='1'%3E%3Cstop offset='0%25' stop-color='%230d1321'/%3E%3Cstop offset='1' stop-color='%231f6f78'/%3E%3C/linearGradient%3E%3C/defs%3E%3Crect width='256' height='256' rx='56' fill='url(%23g)'/%3E%3Ccircle cx='128' cy='128' r='78' fill='none' stroke='%23d7fff1' stroke-width='18'/%3E%3Cpath d='M128 50v32M128 174v32M50 128h32M174 128h32' stroke='%23d7fff1' stroke-width='16' stroke-linecap='round'/%3E%3Ccircle cx='128' cy='128' r='22' fill='%23ffd166'/%3E%3C/svg%3E"

const (
	donationAddressXMR = "43d6rjLpwHSDo4WL3mU8SPbD8MKsFw2mbcRw9foQMyk7BVWWLLpkwHjND1qDVsYfvbGzXExwC9wbZ8Kjx98PPaWf5496rd9"
	donationAddressBTC = "bc1q36hal5nuf7x004dqm9uskdp8dyfympv45387s9"
	donationAddressETH = "0x57c5582967425cB7B01404C5E940114a61713f53"
)

func prefsContent(pacFileURI string, opts Options) (string, error) {
	homePageURI := "about:blank"
	if opts.HomePagePath != "" {
		var err error
		homePageURI, err = util.FileURI(opts.HomePagePath)
		if err != nil {
			return "", fmt.Errorf("convert start page path to file URI: %w", err)
		}
	}
	profileName := opts.ProfileDisplayName
	if profileName == "" {
		profileName = "i2tor Dedicated Profile"
	}
	base := fmt.Sprintf(
		"user_pref(\"network.proxy.type\", 2);\n"+
			"user_pref(\"network.proxy.autoconfig_url\", %q);\n"+
			"user_pref(\"browser.startup.page\", 1);\n"+
			"user_pref(\"browser.startup.homepage\", %q);\n"+
			"user_pref(\"browser.newtabpage.enabled\", false);\n"+
			"user_pref(\"browser.aboutwelcome.enabled\", false);\n"+
			"user_pref(\"extensions.torbutton.use_nontor_proxy\", true);\n"+
			"user_pref(\"extensions.torbutton.startup\", false);\n"+
			"user_pref(\"extensions.torbutton.test_enabled\", false);\n"+
			"user_pref(\"extensions.torlauncher.prompt_at_startup\", false);\n"+
			"user_pref(\"extensions.torlauncher.start_tor\", false);\n"+
			"user_pref(\"i2tor.profile.display_name\", %q);\n"+
			"user_pref(\"browser.fixup.domainsuffixwhitelist.i2p\", true);\n"+
			"user_pref(\"dom.security.https_only_mode\", false);\n"+
			"user_pref(\"dom.security.https_only_mode_pbm\", false);\n"+
			"user_pref(\"dom.security.https_first\", false);\n"+
			"user_pref(\"dom.security.https_first_pbm\", false);\n",
		pacFileURI,
		homePageURI,
		profileName,
	)
	if !opts.AllowLocalhostAccess {
		return base, nil
	}
	return base + strings.Join([]string{
		`user_pref("network.proxy.allow_hijacking_localhost", true);`,
		`user_pref("network.proxy.no_proxies_on", "");`,
		"",
	}, "\n"), nil
}

func WriteDedicatedProfilePrefs(ctx context.Context, profileDir, pacFilePath string, opts Options) error {
	_ = ctx
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return fmt.Errorf("create dedicated profile directory: %w", err)
	}
	pacURI, err := util.FileURI(pacFilePath)
	if err != nil {
		return fmt.Errorf("convert PAC path to file URI: %w", err)
	}
	target := filepath.Join(profileDir, "user.js")
	tmp, err := os.CreateTemp(profileDir, "user-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp profile prefs: %w", err)
	}
	defer os.Remove(tmp.Name())

	content, err := prefsContent(pacURI, opts)
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write dedicated profile prefs: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close dedicated profile prefs: %w", err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("replace dedicated profile prefs: %w", err)
	}
	return nil
}

func WriteStartPage(ctx context.Context, startPageDir string, opts Options) (string, error) {
	_ = ctx
	if err := os.MkdirAll(startPageDir, 0o755); err != nil {
		return "", fmt.Errorf("create start page directory: %w", err)
	}
	target := filepath.Join(startPageDir, "index.html")
	tmp, err := os.CreateTemp(startPageDir, "start-page-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp start page: %w", err)
	}
	defer os.Remove(tmp.Name())

	profileName := opts.ProfileDisplayName
	if profileName == "" {
		profileName = "i2tor Dedicated Profile"
	}
	logoSrc := writeStartPageLogo(startPageDir)
	content := startPageContent(logoSrc, profileName)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write start page: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close start page: %w", err)
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return "", fmt.Errorf("replace start page: %w", err)
	}
	return target, nil
}

func writeStartPageLogo(startPageDir string) string {
	for _, candidate := range startPageLogoCandidates() {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		target := filepath.Join(startPageDir, "logo.png")
		if err := os.WriteFile(target, data, 0o644); err == nil {
			return "logo.png"
		}
	}
	return fallbackStartPageLogoDataURI
}

func startPageLogoCandidates() []string {
	candidates := make([]string, 0, 4)
	if appDir := os.Getenv("APPDIR"); appDir != "" {
		candidates = append(candidates, filepath.Join(appDir, "i2tor.png"))
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "i2tor.png"))
	}
	candidates = append(candidates,
		filepath.Join(".", "i2tor.png"),
		filepath.Join("..", "i2tor.png"),
		filepath.Join("..", "..", "i2tor.png"),
	)
	return candidates
}

func startPageContent(logoSrc, profileName string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>i2tor</title>
  <style>
    :root { color-scheme: dark; --bg:#07111e; --panel:#0d1b2a; --panel-2:#14263c; --ink:#eff6ff; --muted:#9db2ce; --accent:#72f1b8; --accent-2:#6fd3ff; --line:rgba(159,186,219,.18); --code:#0b1624; }
    * { box-sizing:border-box; }
    body { margin:0; font-family: "Segoe UI", Inter, sans-serif; background:
      radial-gradient(circle at top left, rgba(111,211,255,.16), transparent 28%%),
      radial-gradient(circle at top right, rgba(114,241,184,.12), transparent 24%%),
      linear-gradient(180deg, #040914 0%%, var(--bg) 100%%); color:var(--ink); }
    main { max-width: 980px; margin: 0 auto; padding: 48px 24px 64px; }
    section { background: linear-gradient(180deg, rgba(19,35,55,.92), rgba(11,22,36,.96)); border:1px solid var(--line); border-radius:24px; padding:32px; box-shadow: 0 24px 80px rgba(0,0,0,.35); backdrop-filter: blur(10px); }
    h1 { margin:0 0 10px; font-size:44px; letter-spacing:-.03em; }
    h2 { margin:0 0 12px; font-size:20px; letter-spacing:.02em; }
    p, li { color:var(--muted); line-height:1.6; }
    a { color:var(--accent); text-decoration:none; }
    a:hover { color:var(--accent-2); text-decoration:underline; }
    code { background:var(--code); padding:3px 7px; border-radius:8px; color:#d9ecff; }
    ul { margin:0; padding-left:20px; }
    .pill { display:inline-block; padding:7px 12px; border-radius:999px; background:rgba(114,241,184,.12); border:1px solid rgba(114,241,184,.24); color:var(--accent); font-weight:700; margin-bottom:16px; }
    .hero { display:grid; grid-template-columns: 136px minmax(0, 1fr); gap:24px; align-items:center; margin-bottom:28px; }
    .logo-frame { width:136px; height:136px; padding:12px; border-radius:34px; display:grid; place-items:center; background: linear-gradient(180deg, rgba(255,255,255,.10), rgba(255,255,255,.03)); border:1px solid rgba(255,255,255,.10); box-shadow: inset 0 1px 0 rgba(255,255,255,.05), 0 16px 32px rgba(0,0,0,.28); overflow:hidden; }
    .logo { width:100%%; height:100%%; object-fit:cover; border-radius:26px; display:block; }
    .lede { max-width: 58ch; margin:0; }
    .grid { display:grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap:16px; margin-top:24px; align-items:stretch; }
    .card { background: linear-gradient(180deg, rgba(255,255,255,.03), rgba(255,255,255,.015)); border:1px solid var(--line); border-radius:18px; padding:20px; }
    .eyebrow { display:block; margin-bottom:8px; color:var(--accent-2); font-size:12px; font-weight:700; letter-spacing:.12em; text-transform:uppercase; }
    .wallets { display:grid; gap:12px; margin-top:14px; }
    .wallet { padding:12px; border-radius:14px; background:rgba(3,10,18,.55); border:1px solid rgba(159,186,219,.12); }
    .wallet strong { display:block; margin-bottom:6px; color:var(--ink); font-size:13px; letter-spacing:.08em; text-transform:uppercase; }
    .wallet code { display:block; overflow-wrap:anywhere; }
    @media (max-width: 720px) {
      main { padding: 24px 16px 40px; }
      section { padding:24px; }
      .hero { grid-template-columns: 1fr; }
      .logo-frame { width:120px; height:120px; }
      .grid { grid-template-columns: 1fr; }
      h1 { font-size:34px; }
    }
  </style>
</head>
<body>
<main>
  <section>
    <div class="hero">
      <div class="logo-frame">
        <img class="logo" src="%s" alt="i2tor logo">
      </div>
      <div>
        <div class="pill">i2tor-managed session</div>
        <h1>Tor Browser, routed by i2tor</h1>
        <p class="lede">This browser session is using the dedicated launcher-owned profile <code>%s</code>. i2tor keeps Tor Browser intact, launches the bundled Tor daemon itself, and sends only <code>.i2p</code> traffic to the local I2P proxy.</p>
      </div>
    </div>
    <div class="grid">
      <div class="card">
        <span class="eyebrow">Routing</span>
        <h2>Split by network</h2>
        <ul>
          <li><code>.i2p</code> traffic goes to the local I2P HTTP proxy at <code>127.0.0.1:4444</code>.</li>
          <li>All other traffic goes to Tor SOCKS at <code>127.0.0.1:9150</code>.</li>
        </ul>
      </div>
      <div class="card">
        <span class="eyebrow">Getting Started</span>
        <h2>Useful directories</h2>
        <p>Fresh I2P sessions can take a few minutes before eepsites become consistently reachable. If some <code>.i2p</code> sites fail at first, leave the router running and try again.</p>
        <ul>
          <li><a href="http://tortaxi2dev6xjwbaydqzla77rrnth7yn2oqzjfmiuwn5h6vsk2a4syd.onion/">tor.taxi</a> for Tor service directories and current links.</li>
          <li><a href="http://notbob.i2p">notbob.i2p</a> for I2P service directories and starter links.</li>
        </ul>
      </div>
      <div class="card">
        <span class="eyebrow">Notes</span>
        <h2>Profile isolation</h2>
        <ul>
          <li>This is not a browser fork. It is Tor Browser launched with an i2tor-owned profile and proxy configuration.</li>
          <li>i2tor is an independent project and is not affiliated with, endorsed by, or sponsored by the Tor Project or the I2P Project.</li>
          <li>Your normal Tor Browser and Firefox profiles are not modified.</li>
          <li>Advanced settings like localhost access apply only to this dedicated profile.</li>
        </ul>
      </div>
      <div class="card">
        <span class="eyebrow">Donations</span>
        <h2>Support the network first</h2>
        <p>Tor and I2P make this possible. If you donate, please consider supporting those projects first.</p>
        <ul>
          <li><a href="https://donate.torproject.org/">Donate to Tor</a></li>
          <li><a href="https://i2p.net/en/financial-support/">Donate to I2P</a></li>
        </ul>
        <p>If you're still feeling generous, buy me a beer:</p>
        <div class="wallets">
          <div class="wallet">
            <strong>XMR</strong>
            <code>%s</code>
          </div>
          <div class="wallet">
            <strong>BTC</strong>
            <code>%s</code>
          </div>
          <div class="wallet">
            <strong>ETH</strong>
            <code>%s</code>
          </div>
        </div>
      </div>
    </div>
  </section>
</main>
</body>
</html>
`, logoSrc, profileName, donationAddressXMR, donationAddressBTC, donationAddressETH)
}
