// Package markdown renders Markdown to safe HTML using goldmark.
//
// Used for user-authored content (app descriptions, release notes) and
// developer-authored legal pages (datenschutz, impressum).
//
// Configuration:
//   - GFM enabled (tables, strikethrough, autolinks, task lists)
//   - Hard wraps: single newlines render as <br> (matches old behavior)
//   - Raw HTML escaped (default; do NOT enable html.WithUnsafe)
//   - External links get target="_blank" rel="noopener noreferrer" via AST transformer
package markdown

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// externalLinkTransformer walks the AST after parsing and adds
// target="_blank" rel="noopener noreferrer" to every link whose destination
// is absolute http/https. Internal links (relative paths, fragments) are
// left untouched so in-site navigation stays in the same tab.
type externalLinkTransformer struct{}

func (t *externalLinkTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := n.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		dest := string(link.Destination)
		if !isExternal(dest) {
			return ast.WalkContinue, nil
		}
		link.SetAttributeString("target", []byte("_blank"))
		link.SetAttributeString("rel", []byte("noopener noreferrer"))
		return ast.WalkContinue, nil
	})
}

func isExternal(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// md is the shared goldmark instance — configured once, used from ToHTML.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(util.Prioritized(&externalLinkTransformer{}, 100)),
	),
	goldmark.WithRendererOptions(html.WithHardWraps()),
	// NOTE: html.WithUnsafe() is deliberately NOT set — raw HTML is escaped.
)

// ToHTML converts Markdown to safe HTML. Returns empty string on error
// (which the caller can treat as "no content rendered").
func ToHTML(src string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return buf.String()
}
