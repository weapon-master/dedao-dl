package app

import (
	"fmt"
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

	if coverPath != "" {
		os.Remove(coverPath)
	}
	return nil
}

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

		data, err := request.HTTPGet(imgURL)
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
