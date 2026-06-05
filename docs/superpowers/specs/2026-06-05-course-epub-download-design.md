# 课程 EPUB 下载功能设计

## 背景

`dedao-dl dl <course-id> -t` 目前支持 3 种输出格式：1=mp3、2=PDF、3=markdown。需要新增 `-t 4` 支持将课程内容输出为 epub 格式。

## 目标

- 支持命令 `dedao-dl dl <course-id> -t 4` 下载课程为 epub
- epub 封面为课程封面图片
- epub 目录（TOC）分两层：模块（第一层）+ 文章（第二层），文章嵌套在对应模块下
- 复用现有 `HtmlToEpub` + `go-epub` 基础设施

## 数据结构映射

| 得到数据 | epub 映射 |
|---|---|
| `CourseInfo.ChapterList` ([]Chapter) | 第一层 TOC，每个 Chapter = `AddSection` |
| `ArticleBase`（属于某 ChapterID） | 第二层 TOC，`AddSubSection` 嵌套在对应 Chapter 下 |
| `CourseInfo.ClassInfo.Logo` | epub 封面图片 |
| `CourseInfo.ClassInfo.Name` | epub 标题 |
| `CourseInfo.ClassInfo.LecturerNameAndTitle` | epub 作者 |

无模块的课程（`FlatArticleList` 非空），所有文章作为顶层 section。

## 方案

采用方案 A：复用 `HtmlToEpub` + 新增 `ContentsToHTML` 函数。

## 修改文件

### 1. `cmd/download.go`

- 更新 `-t` flag 描述，新增 `4:epub文档`
- `CourseDownload.Download()` switch 新增 `case 4`
- 调用 `app.DownloadEpubCourse(d, course, path)`

### 2. `cmd/app/download.go`

新增两个函数：

#### `ContentsToHTML(contents []services.Content) string`

将 `[]services.Content` 直接转为 HTML。内容类型映射：

| Content.Type | HTML 输出 |
|---|---|
| `audio` | `<h1>{title}</h1>` |
| `header` | `<h{level}>{text}</h{level}>` |
| `paragraph` | `<p>{inline html}</p>` |
| `blockquote` | `<blockquote><p>{line}</p></blockquote>` |
| `list` | `<ul><li>{item}</li></ul>` |
| `elite` | `<h2>划重点</h2><p>{text}</p>` |
| `image` | `<img src="{url}" alt="{legend}">` |
| `label-group` | `<h2><code>{text}</code></h2>` |

inline 元素：bold → `<strong>`，highlight → `<em>`。

#### `DownloadEpubCourse(d *CourseDownload, course *services.CourseInfo, path string) error`

核心流程：

1. 从 `course.ChapterList` 获取模块列表
2. 创建 `HtmlToEpub`，设置封面、标题、作者
3. 遍历模块：
   - `AddSection` 添加模块
   - 用 `ArticleListByEnId(enid, count, chapter.IDStr)` 获取模块下文章（`ArticleList` API 的 `chapterID` 参数接受 string）
   - 对每篇文章：`courseArticleDetailByEnid` → `ContentsToHTML` → `AddSubSection`
4. 无模块时：所有文章作为顶层 section
5. 写入 epub 文件

### 3. 输出路径

`output/<课程名>/EPUB/<课程名>.epub`

## 封面处理

优先使用 `course.ClassInfo.Logo`，回退到 `course.ClassInfo.SquareImg`。复用 `HtmlToEpub.setCover()` 逻辑，传入本地图片路径。

## 进度输出与错误处理

- 每篇文章打印 `正在生成文件：【xxx】完成/失败/已存在`
- 单篇文章失败时打印错误，继续处理下一篇
- 最终生成 epub 时打印一次整体进度

## 不做的事

- 不修改现有 mp3/PDF/markdown 下载逻辑
- 不添加课程评论（`-c` flag）到 epub 输出（可后续迭代）
- 不合并课程（`-m` flag）到 epub 输出（epub 本身就是合集）
- 不处理音频内容嵌入（仅文本+图片）
