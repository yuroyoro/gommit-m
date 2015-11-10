package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/codegangsta/cli"
	"github.com/mattn/go-runewidth"

	"github.com/fatih/color"
)

type commit struct {
	Repo      string `json:"repo"`
	RepoURL   string `json:"repo_url"`
	Sha1      string `json:"sha1"`
	CommitURL string `json:"commit_url"`
	Message   string `json:"message"`
}

type QueryResult struct {
	Commits     []*commit
	ResultCount string
	TotalPages  string
}

type JsonFormat struct {
	Commits []*commit `json:"commits"`
	Error   string    `json:"error"`
}

func main() {
	app := cli.NewApp()
	app.Name = "gommit-m"
	app.Usage = "Command Line Client for commit-m (http://commit-m.minamijoyo.com)"
	app.ArgsUsage = "keyword [page]"
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "json",
			Usage: "output as json",
		},
	}

	app.Action = func(c *cli.Context) {
		keyword := c.Args().First()
		page := 1
		if givenPage := c.Args().Get(1); givenPage != "" {
			if optPage, err := strconv.Atoi(givenPage); err == nil {
				page = optPage
			}
		}

		if keyword == "" {
			cli.ShowAppHelp(c)
			os.Exit(1)
		}

		crawl(keyword, page)
	}

	app.Run(os.Args)
}

func buildUrl(keyword string, page int) string {
	return fmt.Sprintf("http://commit-m.minamijoyo.com/commits/search?keyword=%s&page=%d", url.QueryEscape(keyword), page)
}

func crawl(url string) (QueryResult, error) {
	commits := []*commit{}
	doc, err := goquery.NewDocument(url)
	if err != nil {
		return QueryResult{
			Commits:     commits,
			ResultCount: "",
			TotalPages:  "",
		}, err

	}
	doc.Find("table.table tr").Each(func(_ int, line *goquery.Selection) {
		cellsTxt := [3]string{"", "", ""}
		hrefIndex := 0
		cellsHref := [2]string{"", ""}
		line.Find("td").Each(func(i int, s *goquery.Selection) {
			cellsTxt[i] = s.Text()
			s.Find("a").Each(func(_ int, s *goquery.Selection) {
				href, _ := s.Attr("href")
				if href != "" {
					cellsHref[hrefIndex] = href
					hrefIndex += 1
				}
			})
		})
		commit := commit{
			Message:   strings.TrimSpace(cellsTxt[0]),
			Repo:      cellsTxt[1],
			RepoURL:   cellsHref[0],
			Sha1:      cellsTxt[2],
			CommitURL: cellsHref[1],
		}
		if commit.Sha1 != "" {
			commits = append(commits, &commit)
		}
	})
	return QueryResult{
		Commits:     commits,
		ResultCount: getResultCount(doc),
		TotalPages:  getTotalPages(doc),
	}, nil
}

func getResultCount(doc *goquery.Document) string {
	results := ""
	pattern := regexp.MustCompile("(\\d+) results")
	doc.Find("div.container").Each(func(i int, s *goquery.Selection) {
		for c := s.Nodes[0].FirstChild; c != nil; c = c.NextSibling {
			if c.Type == 1 {
				matches := pattern.FindStringSubmatch(c.Data)
				if len(matches) > 0 {
					results = matches[0]
					break
				}
			}
		}
	})
	return results
}

func getTotalPages(doc *goquery.Document) string {
	pages := doc.Find("ul.pagination li.next_page").Prev().Text()
	if pages == "" {
		pages = "1"
	}

	return pages

}

func maxRepoWidth(commits []*commit) int {
	width := 0
	for _, c := range commits {
		count := runewidth.StringWidth(c.Repo)
		if count > width {
			width = count
		}
	}
	return width
}

func maxMessageWidth(commits []*commit) int {
	width := 0
	for _, c := range commits {
		count := runewidth.StringWidth(c.Message)
		if count > width {
			width = count
		}
	}
	return width
}

func maxURLWidth(commits []*commit) int {
	width := 0
	for _, c := range commits {
		count := utf8.RuneCountInString(c.CommitURL)
		if count > width {
			width = count
		}
	}
	return width
}

func showResult(result QueryResult, url, keyword string, page int) {
	commits := result.Commits
	if len(commits) == 0 {
		fmt.Println("No Results Found.")
		fmt.Printf("  url: %s\n\n", url)
		return
	}
	fmt.Printf("Search Result : %s : %d/%s pages\n",
		result.ResultCount,
		page,
		result.TotalPages,
	)
	fmt.Printf("  url: %s\n\n", url)

	repoWidth := maxRepoWidth(commits)
	repoFmt := fmt.Sprintf("%%-%ds", repoWidth)

	urlWidth := maxURLWidth(commits)
	urlFmt := fmt.Sprintf("%%-%ds", urlWidth)

	msgWidth := maxMessageWidth(commits)

	fmt.Fprintf(color.Output, " %s | %s | %s | message \n",
		color.BlueString(repoFmt, "Repository"),
		color.CyanString("%-7s", "sha1"),
		fmt.Sprintf(urlFmt, "url"),
	)
	fmt.Println(strings.Repeat("-", repoWidth+msgWidth+urlWidth+18))

	for _, c := range commits {
		fmt.Fprintf(color.Output, " %s | %7s | %s | %s\n",
			color.BlueString(repoFmt, c.Repo),
			color.CyanString(c.Sha1),
			fmt.Sprintf(urlFmt, c.CommitURL),
			highlightWords(c.Message, keyword),
		)
	}
}

func showResultAsJson(result QueryResult, err error) {
	enc := json.NewEncoder(os.Stdout)
	if err != nil {
		enc.Encode(JsonFormat{Commits: []*commit{}, Error: err.Error()})
		return
	}
	err = enc.Encode(JsonFormat{Commits: result.Commits, Error: ""})
	if err != nil {
		fmt.Print(err)
	}
}

func highlightWords(message, keyword string) string {
	words := []string{}
	for _, word := range strings.Fields(keyword) {
		words = append(words, regexp.QuoteMeta(word))
	}

	pattern := regexp.MustCompile(strings.Join(words, "|"))
	return pattern.ReplaceAllStringFunc(message, func(s string) string {
		return color.YellowString(s)
	})
}
