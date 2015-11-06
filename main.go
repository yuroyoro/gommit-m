package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/codegangsta/cli"

	"github.com/fatih/color"
)

type commit struct {
	Repo      string
	RepoURL   string
	Sha1      string
	CommitURL string
	Message   string
}

func main() {
	app := cli.NewApp()
	app.Name = "gommit-m"
	app.Usage = "Command Line Client for commit-m (http://commit-m.minamijoyo.com)"
	app.ArgsUsage = "keyword [page]"
	app.HideHelp = true

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

func crawl(keyword string, page int) {
	url := fmt.Sprintf("http://commit-m.minamijoyo.com/commits/search?keyword=%s&page=%d", url.QueryEscape(keyword), page)

	doc, err := goquery.NewDocument(url)
	if err != nil {
		fmt.Println(err)
		return
	}

	commits := []*commit{}
	doc.Find("table.table tr").Each(func(_ int, s *goquery.Selection) {
		cells := []string{}
		s.Find("td").Each(func(_ int, s *goquery.Selection) {
			cells = append(cells, s.Text())
			s.Find("a").Each(func(_ int, s *goquery.Selection) {
				href, _ := s.Attr("href")
				if href != "" {
					cells = append(cells, href)
				}
			})
		})

		if len(cells) < 5 {
			return
		}

		commit := commit{
			Message:   strings.TrimSpace(cells[0]),
			Repo:      cells[1],
			RepoURL:   cells[2],
			Sha1:      cells[3],
			CommitURL: cells[4],
		}
		commits = append(commits, &commit)
	})

	if len(commits) == 0 {
		fmt.Println("No Results Found.")
		fmt.Printf("  url: %s\n\n", url)
		return
	}

	fmt.Printf("Search Result : %s : %d/%s pages\n",
		getResultCount(doc),
		page,
		getTotalPages(doc),
	)
	fmt.Printf("  url: %s\n\n", url)
	showResult(commits, keyword)

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
	return doc.Find("ul.pagination li.next_page").Prev().Text()

}

func maxRepoWidth(commits []*commit) int {
	width := 0
	for _, c := range commits {
		count := utf8.RuneCountInString(c.Repo)
		if count > width {
			width = count
		}
	}
	return width
}

func maxMessageWidth(commits []*commit) int {
	// TODO consider width of East Asian Characters.
	// https://github.com/moznion/go-unicode-east-asian-width
	width := 0
	for _, c := range commits {
		count := utf8.RuneCountInString(c.Message)
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

func showResult(commits []*commit, keyword string) {

	repoWidth := maxRepoWidth(commits)
	repoFmt := fmt.Sprintf("%%-%ds", repoWidth)

	urlWidth := maxURLWidth(commits)
	urlFmt := fmt.Sprintf("%%-%ds", urlWidth)

	msgWidth := maxMessageWidth(commits)

	fmt.Printf(" %s | %s | %s | message \n",
		color.BlueString(repoFmt, "Repository"),
		color.CyanString("%-7s", "sha1"),
		fmt.Sprintf(urlFmt, "url"),
	)
	fmt.Println(strings.Repeat("-", repoWidth+msgWidth+urlWidth+18))

	for _, c := range commits {
		fmt.Printf(" %s | %7s | %s | %s\n",
			color.BlueString(repoFmt, c.Repo),
			color.CyanString(c.Sha1),
			fmt.Sprintf(urlFmt, c.CommitURL),
			highlightWords(c.Message, keyword),
		)
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
