package main

import (
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/goccy/go-yaml"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	fiber_html "github.com/gofiber/template/html/v2"
	"golang.org/x/net/html"

	"runtime/debug"
)

//go:embed views/*
var viewsFS embed.FS

//go:embed public/*
var publicFS embed.FS

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
	ReposDir    string `yaml:"reposDir"`
	HostPort    string `yaml:"hostPort"`
}

var GlobalAppConfig = Config{
	GlobalTitle: "",
	Logo:        "/public/logo.svg",
	Favicon:     "/public/logo.svg",
	ReposDir:    "repos",
	HostPort:    "127.0.0.1:3000",
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

		/*
			projectHref := ""
			readmeFile := path.Join(reposPath, projectEntry.Name(), "README.md")
			if exists, _ := FileExists(readmeFile); exists {
				projectHref = path.Join("/", projectEntry.Name(), "README.md")
			}
		*/
		readmeFile := path.Join(reposPath, projectEntry.Name(), "README.md")
		projectHref := path.Join("/", projectEntry.Name(), "README.md")

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

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		log.Println("Failed to read build info")
	} else {
		log.Println(bi)
	}


	return nil
}

func RenderPage(c fiber.Ctx, markdownRaw string) error {
	var err error
	var markdownHTML string
	var title string = "Untitled"
	if markdownRaw == "" {
		filePath := path.Join(GlobalAppConfig.ReposDir, c.Path())
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
		return c.Redirect().To("/error?message=" + err.Error())
	}
	markdownNode, err := html.Parse(strings.NewReader(markdownHTML))

	var headings = make([]Link, 0)
	if err == nil {
		markdownDoc := goquery.NewDocumentFromNode(markdownNode)
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
	}

	return c.Render("views/page", fiber.Map{
		"title":           title,
		"html":            markdownHTML,
		"version":         "1.0.2-beta",
		"headings":        headings,
		"projects":        projects,
		"currentHref":     c.Path(),
		"currentProject":  strings.Split(c.Path()[1:], "/")[0],
		"globalAppConfig": GlobalAppConfig,
	})
}

func RedirectToProject(c fiber.Ctx, project string) error {
	return c.Redirect().To(path.Join("/", project, "README.md"))
}

func main() {
	loadGlobalAppConfig("config.yaml")
	makeProjects(GlobalAppConfig.ReposDir)

	engine := fiber_html.NewFileSystem(http.FS(viewsFS), ".html")
	engine.AddFunc(
		"unescape", func(s string) template.HTML {
			return template.HTML(s)
		},
	)

	app := fiber.New(fiber.Config{Views: engine})

	app.Get("/git/checkout/:project/:commit", func(c fiber.Ctx) error {
		project := c.Params("project")
		commit := c.Params("commit")
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

	app.Get("/git/submodule/update", func(c fiber.Ctx) error {
		cmd := exec.Command("git", "submodule", "update", "--recursive", "--remote")
		cmd.CombinedOutput()
		makeProjects(GlobalAppConfig.ReposDir)
		return c.Redirect().To("/")
	})

	app.Get("/git/pull/:project", func(c fiber.Ctx) error {
		project := c.Params("project")
		cmd := exec.Command("git", "pull")
		cmd.Dir = path.Join(GlobalAppConfig.ReposDir, project)
		cmd.CombinedOutput()
		makeProjects(GlobalAppConfig.ReposDir)
		return RedirectToProject(c, project)
	})

	app.Get("/", func(c fiber.Ctx) error {
		return RenderPage(c, "# Index")
	})

	app.Get("/*.md", func(c fiber.Ctx) error {
		return RenderPage(c, "")
	})

	app.Get("/public*", static.New("./public"))

	app.Get("/public*", static.New("./public", static.Config{
		FS: publicFS,
	}))

	app.Get("/*", static.New(GlobalAppConfig.ReposDir, static.Config{
		Browse: true,
	}))

	app.Get("/:project/README.md", func(c fiber.Ctx) error {
		project := c.Params("project")
		return RenderPage(c, "# "+project+"\n> README.md does not exist for this project!")
	})

	app.Get("/__favicon", func(c fiber.Ctx) error {
		return c.SendFile(GlobalAppConfig.Favicon)
	})

	app.Get("/__logo", func(c fiber.Ctx) error {
		return c.SendFile(GlobalAppConfig.Logo)
	})

	app.Use(func(c fiber.Ctx) error {
		return RenderPage(c, "# 404 Not Found\n`"+c.Path()+"`")
	})

	log.Fatal(app.Listen(GlobalAppConfig.HostPort))
}
