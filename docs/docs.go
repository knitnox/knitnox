// Package docs embeds the guides shown in the app under Help.
// To add a guide: put the Markdown file here, list it below, and add it to
// helpDocs in internal/web/help.go.
package docs

import "embed"

//go:embed USER_GUIDE.md FEATURES.md INSTALL.md DEVELOPER_GUIDE.md BUILD_GUIDE.md
var FS embed.FS
