# Course EPUB Download Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `-t 4` epub output format to `dedao-dl dl` command, with two-layer TOC (module + article).

**Architecture:** Use `go-epub` library directly. Add `ContentsToHTML` to convert course content to HTML, and `DownloadEpubCourse` to orchestrate the epub build. Modules map to `AddSection`, articles map to `AddSubSection`. Image downloading uses the project's `request` package for consistency.

**Tech Stack:** Go, github.com/bmaupin/go-epub@v1.1.0, github.com/yann0917/dedao-dl/request, github.com/yann0917/dedao-dl/utils

---

## File Structure

| Action | File | Responsibility |
|---|---|---|
| Modify | `cmd/download.go:150` | Add `-t 4` flag description |
| Modify | `cmd/app/download.go:71` | Add `case 4` in `CourseDownload.Download()` |
| Add | `cmd/app/download_epub.go` | New file: `ContentsToHTML`, `DownloadEpubCourse`, and helpers |

The epub download logic is isolated in a new file `cmd/app/download_epub.go` to keep `download.go` focused.

---

### Task 1: Add `ContentsToHTML` function

**Files:**
- Create: `cmd/app/download_epub.go`

- [ ] **Step 1: Create `download_epub.go` with `ContentsToHTML`**

```go
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
```

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./cmd/app/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add cmd/app/download_epub.go
git commit -m "feat: add ContentsToHTML for course epub generation"
```

---

### Task 2: Add `DownloadEpubCourse` and helpers

**Files:**
- Modify: `cmd/app/download_epub.go` (append to file created in Task 1)

Append all remaining functions to `cmd/app/download_epub.go`. First update the import block to include all needed imports, then add `DownloadEpubCourse`, `downloadCoverImage`, and `replaceImageSources`.

- [ ] **Step 1: Update imports to full set**

Replace the import block in `cmd/app/download_epub.go` with:

```go
import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/bmaupin/go-epub"
	"github.com/yann0917/dedao-dl/request"
	"github.com/yann0917/dedao-dl/services"
	"github.com/yann0917/dedao-dl/utils"
)
```

- [ ] **Step 2: Append `DownloadEpubCourse` function**

```go
func DownloadEpubCourse(d *CourseDownload, course *services.CourseInfo, path string) error {
	enid := course.ClassInfo.Enid
	title := course.ClassInfo.Name
	author := course.ClassInfo.LecturerNameAndTitle

	book := epub.NewEpub(title)
	book.SetAuthor(author)
	book.SetDescription(course.ClassInfo.Highlight)

	coverPath, err := downloadCoverImage(course)
	if err == nil && coverPath != "" {
		cover, imgErr := book.AddImage(coverPath, "cover.jpg")
		if imgErr == nil {
			book.SetCover(cover, "")
		}
		os.Remove(coverPath)
	}

	imageDir, err := utils.Mkdir(OutputDir, utils.FileName(title, ""), "EPUB", "images")
	if err != nil {
		return err
	}

	if len(course.ChapterList) > 0 {
		for _, chapter := range course.ChapterList {
			chapterTitle := chapter.Name
			if chapterTitle == "" {
				chapterTitle = fmt.Sprintf("模块 %d", chapter.OrderNum)
			}

			chapterHTML := fmt.Sprintf("<h1>%s</h1>\n", chapterTitle)
			sectionFilename, err := book.AddSection(chapterHTML, chapterTitle, fmt.Sprintf("chapter_%d", chapter.ID), "")
			if err != nil {
				fmt.Printf("\033[31;1m添加模块失败: %s\033[0m\n", err.Error())
				continue
			}

			count := chapter.PhaseNum
			if count == 0 {
				count = course.ClassInfo.CurrentArticleCount
			}
			articles, err := ArticleListByEnId(enid, count, chapter.IDStr)
			if err != nil {
				fmt.Printf("\033[31;1m获取文章列表失败: %s\033[0m\n", err.Error())
				continue
			}

			for _, article := range articles.List {
				if d.AID > 0 && article.ID != d.AID {
					continue
				}
				if err := addArticleToBook(book, d, &article, imageDir, sectionFilename, true); err != nil {
					continue
				}
			}
		}
	} else {
		count := course.ClassInfo.CurrentArticleCount
		if count == 0 {
			count = course.ClassInfo.PhaseNum
		}
		articles, err := ArticleListByEnId(enid, count, "")
		if err != nil {
			return err
		}

		for _, article := range articles.List {
			if d.AID > 0 && article.ID != d.AID {
				continue
			}
			if err := addArticleToBook(book, d, &article, imageDir, "", false); err != nil {
				continue
			}
		}
	}

	fileName, err := utils.FilePath(filepath.Join(path, utils.FileName(title, "")), "epub", false)
	if err != nil {
		return err
	}

	fmt.Printf("正在生成文件：【\033[37;1m%s\033[0m】 ", fileName)
	if err = book.Write(fileName); err != nil {
		fmt.Printf("\033[31;1m失败: %s\033[0m\n", err.Error())
		return err
	}
	fmt.Printf("\033[32;1m完成\033[0m\n")
	return nil
}
```

- [ ] **Step 3: Append `addArticleToBook` helper**

Extracts article detail, converts to HTML, and adds as section or subsection.

```go
func addArticleToBook(book *epub.Epub, d *CourseDownload, article *services.ArticleIntro, imageDir string, parentFilename string, asSubSection bool) error {
	articleName := article.Title
	if d.IsOrder {
		articleName = fmt.Sprintf("%03d.%s", article.OrderNum, articleName)
	}
	fmt.Printf("正在生成文件：【\033[37;1m%s\033[0m】 ", articleName)

	detail, err := courseArticleDetailByEnid(article.Enid)
	if err != nil {
		fmt.Printf("\033[31;1m失败: %s\033[0m\n", err.Error())
		return err
	}

	var content []services.Content
	if err := jsoniter.UnmarshalFromString(detail.Content, &content); err != nil {
		fmt.Printf("\033[31;1m失败: %s\033[0m\n", err.Error())
		return err
	}

	htmlContent := ContentsToHTML(content)
	htmlContent = replaceImageSources(htmlContent, imageDir, book)

	if asSubSection {
		_, err = book.AddSubSection(parentFilename, htmlContent, articleName, fmt.Sprintf("article_%d", article.ID), "")
	} else {
		_, err = book.AddSection(htmlContent, articleName, fmt.Sprintf("article_%d", article.ID), "")
	}
	if err != nil {
		fmt.Printf("\033[31;1m失败: %s\033[0m\n", err.Error())
		return err
	}
	fmt.Printf("\033[32;1m完成\033[0m\n")
	return nil
}
```

- [ ] **Step 4: Append `downloadCoverImage` helper**

```go
func downloadCoverImage(course *services.CourseInfo) (string, error) {
	coverURL := course.ClassInfo.Logo
	if coverURL == "" {
		coverURL = course.ClassInfo.SquareImg
	}
	if coverURL == "" {
		return "", nil
	}

	data, err := request.HTTPGet(coverURL)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "dedao-cover-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}
```

- [ ] **Step 5: Append `replaceImageSources` with regex-based image handling**

```go
var imgSrcRegex = regexp.MustCompile(`<img src="(https?://[^"]+)"`)

func replaceImageSources(htmlContent string, imageDir string, book *epub.Epub) string {
	matches := imgSrcRegex.FindAllStringSubmatch(htmlContent, -1)
	if len(matches) == 0 {
		return htmlContent
	}

	for i, match := range matches {
		if len(match) < 2 {
			continue
		}
		imgURL := match[1]

		resp, err := http.Get(imgURL)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		uri, _ := url.Parse(imgURL)
		ext := filepath.Ext(uri.Path)
		if ext == "" {
			ext = ".jpg"
		}
		localFile := filepath.Join(imageDir, fmt.Sprintf("img_%03d%s", i, ext))
		if err := os.WriteFile(localFile, data, 0644); err != nil {
			continue
		}

		internalName := fmt.Sprintf("img_%03d%s", i, ext)
		internalRef, err := book.AddImage(localFile, internalName)
		if err != nil {
			continue
		}

		htmlContent = strings.Replace(htmlContent, imgURL, internalRef, 1)
	}

	return htmlContent
}
```

- [ ] **Step 6: Build to verify compilation**

Run: `go build ./cmd/app/`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add cmd/app/download_epub.go
git commit -m "feat: add DownloadEpubCourse with cover and image handling"
```

---

### Task 3: Wire up `case 4` in command entry points

**Files:**
- Modify: `cmd/app/download.go:71-130` — add `case 4` in switch
- Modify: `cmd/download.go:16-17,150` — update Long description and flag description

- [ ] **Step 1: Add `case 4` in `CourseDownload.Download()` switch**

In `cmd/app/download.go`, after the `case 3:` block (ends around line 129), before the closing `}`, add:

```go
		case 4:
			// 下载 EPUB
			path, err := utils.Mkdir(OutputDir, utils.FileName(course.ClassInfo.Name, ""), "EPUB")
			if err != nil {
				return err
			}
			d.ClassName = course.ClassInfo.Name
			if err := DownloadEpubCourse(d, course, path); err != nil {
				return err
			}
```

- [ ] **Step 2: Update Long description in `cmd/download.go`**

Change line 16-17 from:

```go
		Long: `使用 dedao-dl dl 下载已购买课程, 并转换成 PDF & 音频 & markdown
	-t 指定下载格式, 1:mp3, 2:PDF文档, 3:markdown文档, 默认 mp3
```

to:

```go
		Long: `使用 dedao-dl dl 下载已购买课程, 并转换成 PDF & 音频 & markdown & epub
	-t 指定下载格式, 1:mp3, 2:PDF文档, 3:markdown文档, 4:epub文档, 默认 mp3
```

- [ ] **Step 3: Update `-t` flag description in `cmd/download.go`**

Change line 150 from:

```go
	downloadCmd.PersistentFlags().IntVarP(&downloadType, "downloadType", "t", 1, "下载格式, 1:mp3, 2:PDF文档, 3:markdown文档")
```

to:

```go
	downloadCmd.PersistentFlags().IntVarP(&downloadType, "downloadType", "t", 1, "下载格式, 1:mp3, 2:PDF文档, 3:markdown文档, 4:epub文档")
```

- [ ] **Step 4: Build the full binary**

Run: `go build -o dedao-dl .`
Expected: no errors

- [ ] **Step 5: Verify help text**

Run: `./dedao-dl dl --help`
Expected: flag description shows `1:mp3, 2:PDF文档, 3:markdown文档, 4:epub文档`

- [ ] **Step 6: Commit**

```bash
git add cmd/download.go cmd/app/download.go
git commit -m "feat: wire up epub download (-t 4) in dl command"
```

---

### Task 4: Build and smoke test

**Files:** None (testing only)

- [ ] **Step 1: Build**

Run: `go build -o dedao-dl .`
Expected: success

- [ ] **Step 2: Verify command help**

Run: `./dedao-dl dl --help`
Expected: shows `-t` with 4 options including `4:epub文档`

- [ ] **Step 3: Final commit if any fixups needed**

If any issues were found and fixed during smoke test, commit them.
