package main

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed inject/ui/theme.js
var themeScript string

//go:embed inject/ui/toast.js
var toastScript string

//go:embed inject/ui/loading.html
var loadingHTMLTemplate string

//go:embed inject/ui/mobile_pair.html
var mobilePairHTMLTemplate string


//go:embed inject/copy/copy.js
var copyScript string

//go:embed inject/window/isolation.js
var isolationScript string

func getLoadingHTML(isDark bool, message string) string {
	themeClass := ""
	if isDark {
		themeClass = "dark"
	}
	html := strings.ReplaceAll(loadingHTMLTemplate, "{{THEME}}", themeClass)
	html = strings.ReplaceAll(html, "{{MESSAGE}}", message)
	return html
}

func buildInitScript(initConnJSON string, initEmpty bool, isDark bool) string {
	themePref := "light"
	if isDark {
		themePref = "dark"
	}
	emptyStr := fmt.Sprintf("%t", initEmpty)

	return fmt.Sprintf(`
(function() {
	if (!window.location.href.startsWith('http')) return;

	const initConn = %s;
	const initEmpty = %s;
	const themePref = '%s';

	// 1. Theme setup
	%s

	// 2. Toast notification system
	%s

	// 3. Full-row and multi-row copy handler
	%s

	// 4. Per-window isolation & routing
	%s
})();
`, initConnJSON, emptyStr, themePref, themeScript, toastScript, copyScript, isolationScript)
}
