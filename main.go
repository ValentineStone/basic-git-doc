package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/goccy/go-yaml"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	fiber_html "github.com/gofiber/template/html/v2"
	"github.com/gosimple/slug"
	"golang.org/x/net/html"

	"runtime/debug"
)

//go:embed views/*
var viewsFS embed.FS

//go:embed public-embeded/*
var embededFS embed.FS

type Link struct {
	Text string
	Href string
}

type ProjectTOC struct {
	Name  string
	Href  string
	Pages []Link
	Tags  []string
	Tag   string
}

type Config struct {
	GlobalTitle string `yaml:"globalTitle"`
	Logo        string `yaml:"logo"`
	Favicon     string `yaml:"favicon"`
	Readme      string `yaml:"readme"`
	ReposDir    string `yaml:"reposDir"`
	HostPort    string `yaml:"hostPort"`
	Uploads     bool   `yaml:"uploads"`
}

var GlobalAppVersion = "0.0.0-unknown"

var GlobalAppConfig = Config{
	GlobalTitle: "",
	Logo:        "public/logo.svg",
	Favicon:     "public/logo.svg",
	Readme:      "public/README.md",
	ReposDir:    "repos",
	HostPort:    "127.0.0.1:3000",
	Uploads:     false,
}

var projects []ProjectTOC

func MarkdownDocumentTitle(filePath string) (string, error) {
	markdownHTML, err := MarkdownParseFile(filePath)
	if err != nil {
		return "", nil
	}
	markdownNode, err := html.Parse(strings.NewReader(markdownHTML))
	if err != nil {
		return "", nil
	} else {
		markdownDoc := goquery.NewDocumentFromNode(markdownNode)
		title, err := MarkdownDocumentTitleFromDom(markdownDoc)
		if err != nil {
			return "", errors.New("no heading")
		} else {
			return title, nil
		}
	}
}

func MarkdownDocumentTitleFromDom(document *goquery.Document) (string, error) {
	headingsSelection := document.Find("h1")
	if headingsSelection.Length() > 0 {
		return headingsSelection.First().Text(), nil
	} else {
		return "", errors.New("no h1 heading found")
	}
}

func makeProjects(reposPath string) error {

	projects = make([]ProjectTOC, 0)

	entries, err := os.ReadDir(reposPath)
	if err != nil {
		return err
	}

	for _, projectEntry := range entries {
		if !projectEntry.IsDir() {
			continue
		}

		readmeFile := path.Join(reposPath, projectEntry.Name(), "README.md")
		projectHref := path.Join("/", projectEntry.Name())

		projects = append(projects, ProjectTOC{
			Name:  projectEntry.Name(),
			Href:  projectHref,
			Pages: make([]Link, 0),
			Tags:  GitTags(projectEntry.Name()),
			Tag:   GitTag(projectEntry.Name()),
		})
		projectIndex := len(projects) - 1

		docPath := path.Join(reposPath, projectEntry.Name())
		mdFiles, err := FilesList(docPath, ".md$", ".git$")

		if err != nil {
			continue
		}

		for _, mdFile := range mdFiles {
			if mdFile == readmeFile {
				continue
			}
			title, err := MarkdownDocumentTitle(mdFile)
			if err != nil {
				title = path.Base(mdFile)
			}
			projects[projectIndex].Pages = append(projects[projectIndex].Pages, Link{
				Text: title,
				Href: strings.Replace(mdFile, path.Join(reposPath), "", 1),
			})
		}
	}

	return nil

}

func GitExists(project string) bool {
	exists, _ := FileExists(path.Join(GlobalAppConfig.ReposDir, project, ".git"))
	return exists
}

func GitBranch(project string) string {
	if !GitExists(project) {
		return ""
	}
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func GitTag(project string) string {
	if !GitExists(project) {
		return ""
	}
	cmd := exec.Command("git", "describe", "--exact-match", "--tags")
	cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
	out, err := cmd.CombinedOutput()
	if err != nil {
		out = []byte("latest")
	}
	return strings.TrimSpace(string(out))
}

func GitLatestCommit(project string) (string, error) {
	if !GitExists(project) {
		return "", errors.New("not a git repository")
	}
	cmd := exec.Command("git", "log", "--branches", "-1", `--pretty=format:%H`)
	cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	} else {
		return strings.TrimSpace(string(out)), nil
	}
}

func GitTags(project string) []string {
	if !GitExists(project) {
		return []string{}
	}
	cmd := exec.Command("git", "tag")
	cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
	out, err := cmd.CombinedOutput()
	if err != nil {
		out = []byte("")
	}
	tags := strings.Fields(string(out))
	sort.Sort(sort.Reverse(sort.StringSlice(tags)))
	return append([]string{"latest"}, tags...)
}

func loadGlobalAppConfig(file string) error {
	configBytes, err := FileReadBytes(file)
	if err != nil {
		return err
	}
	yaml.Unmarshal(configBytes, &GlobalAppConfig)

	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		GlobalAppVersion = buildInfo.Main.Version
	}

	return nil
}

func RenderPage(c *fiber.Ctx, markdownRaw string) error {
	var err error
	var markdownHTML string
	var title string = "Untitled"
	if markdownRaw == "" {
		filePath := path.Join(GlobalAppConfig.ReposDir, FiberPath(c))
		fileName := path.Base(filePath)
		if exists, _ := FileExists(filePath); !exists {
			return c.Next()
		}
		markdownHTML, err = MarkdownParseFile(filePath)
		title = fileName
	} else {
		markdownHTML = MarkdownParse(markdownRaw)
	}

	if err != nil {
		return RenderError(c, err.Error())
	}
	markdownNode, err := html.Parse(strings.NewReader(markdownHTML))

	var headings = make([]Link, 0)
	if err == nil {
		markdownDoc := goquery.NewDocumentFromNode(markdownNode)

		allHeadingsSelection := markdownDoc.Find("h1, h2, h3, h4, h5, h6")
		allHeadingsSelection.Each(func(i int, s *goquery.Selection) {
			s.SetAttr("id", slug.Make(s.Text()))
		})

		headingsSelection := markdownDoc.Find("h1")
		if headingsSelection.Length() <= 1 {
			headingsSelection = markdownDoc.Find("h2")
		}
		headingsSelection.Each(func(i int, s *goquery.Selection) {
			headings = append(headings, Link{
				Text: s.Text(),
				Href: "#" + s.AttrOr("id", ""),
			})
		})
		markdownTitle, err := MarkdownDocumentTitleFromDom(markdownDoc)
		if err == nil {
			title = markdownTitle
		}

		markdownHTMLNew, err := goquery.OuterHtml(markdownDoc.Selection)
		if err == nil {
			markdownHTML = markdownHTMLNew
		}
	}

	downloadLink := c.Path() + "?download=" + url.QueryEscape(title) + ".md"

	return c.Render("views/page", fiber.Map{
		"title":           title,
		"html":            markdownHTML,
		"version":         GlobalAppVersion,
		"headings":        headings,
		"projects":        projects,
		"currentHref":     FiberPath(c),
		"currentHrefRaw":  c.Path(),
		"currentProject":  strings.Split(FiberPath(c)[1:], "/")[0],
		"globalAppConfig": GlobalAppConfig,
		"downloadLink":    downloadLink,
	})
}

func RedirectToProject(c *fiber.Ctx, project string) error {
	return c.Redirect(path.Join("/", project))
}

func RenderError(c *fiber.Ctx, message string, code ...int) error {
	if len(code) > 0 {
		errorText := http.StatusText(code[0])
		return RenderPage(c, "# "+strconv.Itoa(code[0])+" "+errorText+"\n> "+message)
	} else {
		return RenderPage(c, "# Error\n> "+message)
	}
}

func DownloadMarkdownFile(c *fiber.Ctx, project string, subpath string, downloadName string) error {
	filepath := path.Join(GlobalAppConfig.ReposDir, project, subpath)
	downloadFileName := project + "_" + strings.ReplaceAll(downloadName, " ", "_")

	yamlheaderbytes := []byte(
		"---\n" +
			"project: " + strconv.Quote(project) + "\n" +
			"file:    " + strconv.Quote(subpath) + "\n" +
			"---\n\n",
	)

	filebytes, err := FileReadBytes(filepath)
	if err != nil {
		return RenderError(c, err.Error())
	}

	c.Attachment(downloadFileName)
	return c.Send(append(yamlheaderbytes, filebytes...))
}

var versionFlag = flag.Bool("version", false, "print version")
var systemIdFlag = flag.Bool("system-id", false, "print system id")
var hostportFlag = flag.String("hostport", "127.0.0.1:3000", "if defined will be used over config.yaml hostport")

func main() {
	loadGlobalAppConfig("config.yaml")
	flag.Parse()
	if *versionFlag {
		fmt.Println(GlobalAppVersion)
		return
	}
	if *systemIdFlag {
		systemId, err := SystemIdString()
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(systemId)
		}
		return
	}
	if FlagPassed("hostport") {
		GlobalAppConfig.HostPort = *hostportFlag
	}

	makeProjects(GlobalAppConfig.ReposDir)

	engine := fiber_html.NewFileSystem(http.FS(viewsFS), ".html")
	engine.AddFunc(
		"unescape", func(s string) template.HTML {
			return template.HTML(s)
		},
	)
	engine.AddFunc(
		"json", func(v any) template.JS {
			a, _ := json.MarshalIndent(v, "", "  ")
			return template.JS(a)
		},
	)

	app := fiber.New(fiber.Config{
		Views:                 engine,
		DisableStartupMessage: true,
		BodyLimit:             10 * 1024 * 1024, // 10 MB max upload size
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}
			ctx.Status(code)
			return RenderError(ctx, err.Error(), code)
		},
	})

	app.Get("/error", func(c *fiber.Ctx) error {
		message := c.Query("message", "unknown error")
		return RenderPage(c, "# Error\n> "+message)
	})

	app.Get("/git/checkout/:project/:commit", func(c *fiber.Ctx) error {
		project := FiberParam(c, "project")
		commit := FiberParam(c, "commit")
		var cmd *exec.Cmd
		if commit == "latest" {
			latestCommit, err := GitLatestCommit(project)
			if err != nil {
				return RedirectToProject(c, project)
			}
			commit = latestCommit
		}
		cmd = exec.Command("git", "checkout", commit)
		cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
		cmd.CombinedOutput()
		makeProjects(GlobalAppConfig.ReposDir)
		return RedirectToProject(c, project)
	})

	app.Get("/git/submodule/update", func(c *fiber.Ctx) error {
		cmd := exec.Command("git", "submodule", "update", "--recursive", "--remote")
		cmd.CombinedOutput()
		makeProjects(GlobalAppConfig.ReposDir)
		return c.Redirect("/")
	})

	app.Get("/git/pull/:project", func(c *fiber.Ctx) error {
		project := FiberParam(c, "project")
		cmd := exec.Command("git", "pull")
		cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
		cmd.CombinedOutput()
		makeProjects(GlobalAppConfig.ReposDir)
		return RedirectToProject(c, project)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		if exists, _ := FileExists(GlobalAppConfig.Readme); exists {
			markdownRaw, err := FileReadString(GlobalAppConfig.Readme)
			if err != nil {
				return RenderPage(c, "# Index\n> "+err.Error())
			} else {
				return RenderPage(c, markdownRaw)
			}
		} else {
			return RenderPage(c, "# Index")
		}
	})

	app.Static("/public", "public", fiber.Static{
		Browse: true,
	})

	app.Use("/public", filesystem.New(filesystem.Config{
		Root:       http.FS(embededFS),
		PathPrefix: "public-embeded",
		Browse:     true,
	}))

	app.Get("/:project/*", func(c *fiber.Ctx) error {
		ok, err := LicenseCheck()
		if !ok {
			return RenderError(c, err.Error())
		}

		project := FiberParam(c, "project")
		subpath := FiberParam(c, "*")
		if subpath == "" {
			return c.Redirect(path.Join(project, "README.md"))
		}

		filepath := path.Join(GlobalAppConfig.ReposDir, project, subpath)
		if ok, _ := FileExists(filepath); !ok {
			return c.Next()
		}

		if strings.HasSuffix(strings.ToLower(subpath), ".md") {
			if c.Query("download") != "" {
				return DownloadMarkdownFile(c, project, subpath, c.Query("download"))
			}

			return RenderPage(c, "")
		}

		return c.SendFile(filepath)
	})

	app.Post("/api/upload", func(c *fiber.Ctx) error {
		if !GlobalAppConfig.Uploads {
			return errors.New("upload feature not enabled")
		}

		file, err := c.FormFile("file")
		if err != nil {
			return RenderError(c, err.Error())
		}
		timestamp := time.Now().UTC().Format("2006-01-02-15-04-05")

		destination := fmt.Sprintf("./upload/%s %s", timestamp, file.Filename)
		err = c.SaveFile(file, destination)
		if err != nil {
			return RenderError(c, err.Error())
		}

		if redirect := c.FormValue("redirect"); redirect == "" {
			return RenderPage(c, "# Success\nFile uploaded")
		} else {
			return RenderPage(c, "# Success\nFile uploaded, [go back]("+redirect+")")
			//return c.Redirect(redirect)
		}
	})

	app.Get("/:project/README.md", func(c *fiber.Ctx) error {
		ok, err := LicenseCheck()
		if !ok {
			return RenderError(c, err.Error())
		}
		project := FiberParam(c, "project")
		return RenderPage(c, "# "+project+"\n> README.md does not exist for this project!")
	})

	app.Use(func(c *fiber.Ctx) error {
		return RenderPage(c, "# 404 Not Found\n`"+FiberPath(c)+"`")
	})

	app.Hooks().OnListen(func(listenData fiber.ListenData) error {
		if fiber.IsChild() {
			return nil
		}
		scheme := "http"
		if listenData.TLS {
			scheme = "https"
		}
		host := listenData.Host
		comment := ""
		if host == "0.0.0.0" {
			host = "127.0.0.1"
			comment = "(listening on 0.0.0.0)"
		}
		fmt.Println("Docs running on " + scheme + "://" + host + ":" + listenData.Port + " " + comment)
		return nil
	})

	log.Fatal(app.Listen(GlobalAppConfig.HostPort))
}
