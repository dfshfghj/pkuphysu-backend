package utils

import (
	"bytes"

	"github.com/88250/lute"
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"

	log "github.com/sirupsen/logrus"
)

func MarkdownToHtml(markdown string) string {
	luteEngine := lute.New()
	html := luteEngine.MarkdownStr("", markdown)
	p := bluemonday.UGCPolicy()
	p.AllowElements("iframe")
	p.AllowAttrs("src", "href", "style", "class", "id").Globally()
	html = p.Sanitize(html)
	log.Debug("markdown: ", markdown)
	log.Debug("html: ", html)

	return html
}

func HtmlToText(html string) string {
	doc, err := goquery.NewDocumentFromReader(
		bytes.NewReader([]byte(html)))
	if err != nil {
		log.Error(err)
		return ""
	}

	text := doc.Text()
	log.Debug("html: ", html)
	log.Debug("text: ", text)

	return text
}

func MarkdownToText(markdown string) string {
	html := MarkdownToHtml(markdown)
	text := HtmlToText(html)
	return text
}
