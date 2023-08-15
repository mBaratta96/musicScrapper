package main

import (
	"cli"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"scraper"
	"strconv"

	"github.com/wk8/go-ordered-map/v2"
)

type gotData struct {
	credits bool
	reviews bool
	similar bool
}

func checkIndex(index int) int {
	if index == -1 {
		os.Exit(0)
	}
	return index
}

// RYM is the website that requires more configuration (cookies, credits scraping, etc...)
// However, we still make similar operations for both of the two websites: search an artist,
// select an artist, select an album, get album data. The similarity of these operation is what led to
// implement the scraper.Scraper interface.
func app(s scraper.Scraper) {
	data := scraper.ScrapeData(s.SearchBand)
	index := -1
	if len(data.Links) == 0 {
		fmt.Println("No result for your search")
	} else {
		index = cli.PrintTable(data.Rows, data.Columns.Title, data.Columns.Width)
	}
	index = checkIndex(index)
	s.SetLink(data.Links[index])
	// Scrape the albums of an artist
	data = scraper.ScrapeData(s.AlbumList)
	for true {
		cli.CallClear()
		cli.PrintMap(s.StyleColor(), data.Metadata)
		index = checkIndex(cli.PrintTable(data.Rows, data.Columns.Title, data.Columns.Width))
		s.SetLink(data.Links[index])
		// Scrape albm data
		albumData := scraper.ScrapeData(s.Album)
		cli.CallClear()
		if albumData.Image != nil {
			cli.PrintImage(albumData.Image)
		}
		cli.PrintMap(s.StyleColor(), albumData.Metadata)
		cli.PrintLink(data.Links[index])
		_ = checkIndex(cli.PrintTable(albumData.Rows, albumData.Columns.Title, albumData.Columns.Width))
		var creditsData *orderedmap.OrderedMap[string, string]
		var reviewData scraper.ScrapedData
		var similData scraper.ScrapedData
		var flags gotData = gotData{}
		for true {
			listIndex := cli.PrintList(s.ListChoices())
			if listIndex == "Exit" {
				os.Exit(0)
			}
			goingBack := false
			switch listIndex {
			case "Go back":
				goingBack = true
			case "Show credits":
				if !flags.credits {
					creditsData = s.Credits()
					flags.credits = true
				}
				cli.PrintMap(s.StyleColor(), creditsData)

			case "Show reviews":
				if !flags.reviews {
					reviewData = scraper.ScrapeData(s.ReviewsList)
					flags.reviews = true
				}
				reviewIndex := checkIndex(
					cli.PrintTable(reviewData.Rows, reviewData.Columns.Title, reviewData.Columns.Width),
				)
				if len(reviewData.Links) > 0 {
					cli.PrintReview(reviewData.Links[reviewIndex])
				}
			case "Set rating":
				var rating string
				for true {
					fmt.Println("Insert rating (0 to 10):")
					if _, err := fmt.Scanln(&rating); err != nil {
						panic(err)
					}
					if i, err := strconv.Atoi(rating); err == nil && i >= 0 && i <= 10 {
						break
					}
					fmt.Println("Wrong rating value")
				}
				id, _ := albumData.Metadata.Get("ID")
				s.AdditionalFunctions()["Set rating"].(func(string, string))(rating, id)
			case "Get similar artists":
				if !flags.similar {
					id, _ := albumData.Metadata.Get("ID")
					s.SetLink(fmt.Sprintf("https://www.metal-archives.com/band/ajax-recommendations/id/%s", id))
					similData = scraper.ScrapeData(
						s.AdditionalFunctions()["Get similar artists"].(func(*scraper.ScrapedData) ([]int, []string)),
					)
					flags.similar = true
				}
				similIndex := checkIndex(cli.PrintTable(similData.Rows, similData.Columns.Title, similData.Columns.Width))
				if similIndex < len(similData.Links) { // go to new artist and start again
					s.SetLink(similData.Links[similIndex])
					data = scraper.ScrapeData(s.AlbumList)
					goingBack = true
				} else { // similIndex is the "Go back" option. Get back to current artist and do nothing
					s.SetLink(data.Links[index])
				}
			}
			if goingBack {
				break
			}
		}
	}
}

func main() {
	website := flag.String("website", "", "Desired Website ('metallum' or 'rym')")
	rymCredits := flag.Bool("credits", false, "Display RYM credits")
	expand := flag.Bool("expand", false, "Expand RYM albums")
	flag.Parse()
	if len(flag.Args()) == 0 {
		os.Exit(1)
	}
	if !(*website == "metallum" || *website == "rym") {
		fmt.Println("Wrong website")
		os.Exit(1)
	}
	search := flag.Arg(0)

	configFolder, err := os.UserConfigDir()
	if err != nil {
		fmt.Println("Cannot determine config folder")
		os.Exit(1)
	}
	configFilePath := filepath.Join(configFolder, "musicScraper", "config.json")

	if *website == "metallum" {
		app(&scraper.Metallum{Link: search})
	}
	if *website == "rym" {
		r := &scraper.RateYourMusic{}
		r.Link = search
		r.GetCredits = *rymCredits
		r.Expand = *expand
		config, _ := scraper.ReadUserConfiguration(configFilePath)
		r.Delay = config.Delay
		if config.Authenticate {
			cacheFolder, err := os.UserCacheDir()
			if err != nil {
				fmt.Println("Cannot determine cache folder")
				os.Exit(1)
			}
			cookieFilePath := filepath.Join(cacheFolder, "musicScraper", "cookie.json")
			if _, err := os.Stat(cookieFilePath); os.IsNotExist(err) {
				r.Login()
				if config.SaveCookies {
					scraper.SaveCookie(r.Cookies, cookieFilePath)
				}
			} else {
				r.Cookies, _ = scraper.ReadCookie(cookieFilePath)
			}
		}
		app(r)
	}
}
