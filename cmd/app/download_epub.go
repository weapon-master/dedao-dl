package app

import (
	"fmt"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/yann0917/dedao-dl/services"
)

func ContentsToHTML(contents []services.Content) string {
	var sb strings.Builder
	for _, content := range contents {
		switch content.Type {
		case "audio":
			title := strings.TrimRight(content.Title, ".mp3")
			sb.WriteString(fmt.Sprintf("<h1>%s</h1>\n", title))
		case "header":
			text := strings.TrimSpace(content.Text)
			if text != "" {
				if content.Level < 1 {
					content.Level = 1
				}
				if content.Level > 6 {
					content.Level = 6
				}
				sb.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", content.Level, text, content.Level))
			}
		case "paragraph":
			res, err := paragraphToHTML(content.Contents)
			if err == nil && res != "" {
				sb.WriteString(res)
			}
		case "blockquote":
			lines := strings.Split(content.Text, "\n")
			sb.WriteString("<blockquote>\n")
			for _, line := range lines {
				sb.WriteString(fmt.Sprintf("<p>%s</p>\n", line))
			}
			sb.WriteString("</blockquote>\n")
		case "list":
			res, err := listToHTML(content.Contents)
			if err == nil && res != "" {
				sb.WriteString(res)
			}
		case "elite":
			sb.WriteString(fmt.Sprintf("<h2>划重点</h2>\n<p>%s</p>\n", content.Text))
		case "image":
			sb.WriteString(fmt.Sprintf("<img src=\"%s\"", content.URL))
			if content.Legend != "" {
				sb.WriteString(fmt.Sprintf(" alt=\"%s\"", content.Legend))
			}
			sb.WriteString(">\n")
		case "label-group":
			sb.WriteString(fmt.Sprintf("<h2><code>%s</code></h2>\n", content.Text))
		}
	}
	return sb.String()
}

func paragraphToHTML(content interface{}) (string, error) {
	tmpJson, err := jsoniter.Marshal(content)
	if err != nil {
		return "", err
	}
	cont := services.Contents{}
	if err := jsoniter.Unmarshal(tmpJson, &cont); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, item := range cont {
		text := strings.TrimSpace(item.Text.Content)
		switch item.Type {
		case "text":
			if item.Text.Bold {
				sb.WriteString(fmt.Sprintf(" <strong>%s</strong> ", text))
			} else if item.Text.Highlight {
				sb.WriteString(fmt.Sprintf(" <em>%s</em> ", text))
			} else {
				sb.WriteString(text)
			}
		}
	}
	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", nil
	}
	return fmt.Sprintf("<p>%s</p>\n", result), nil
}

func listToHTML(content interface{}) (string, error) {
	tmpJson, err := jsoniter.Marshal(content)
	if err != nil {
		return "", err
	}
	var cont []services.Contents
	if err := jsoniter.Unmarshal(tmpJson, &cont); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("<ul>\n")
	for _, item := range cont {
		for _, sub := range item {
			text := strings.TrimSpace(sub.Text.Content)
			switch sub.Type {
			case "text":
				if sub.Text.Bold {
					sb.WriteString(fmt.Sprintf("<li><strong>%s</strong></li>\n", text))
				} else if sub.Text.Highlight {
					sb.WriteString(fmt.Sprintf("<li><em>%s</em></li>\n", text))
				} else {
					sb.WriteString(fmt.Sprintf("<li>%s</li>\n", text))
				}
			}
		}
	}
	sb.WriteString("</ul>\n")
	return sb.String(), nil
}
