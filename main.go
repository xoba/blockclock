package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/caseymrm/menuet"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	go bitcoinStatus()
	menuet.App().RunApplication()
}

type Latest struct {
	Hash   string
	Height int
	Time   int
}

type Price struct {
	Bitcoin map[string]float64
}

type DataSet struct {
	Error   error
	Fetched time.Time
	*Latest
	*Price
}

func fetchData() (out DataSet) {
	out.Fetched = time.Now()
	h, err := getHeight()
	if err != nil {
		out.Error = err
		return
	}
	out.Latest = h
	p, err := getPrice()
	if err != nil {
		out.Error = err
	}
	out.Price = p
	return
}

func setText(s string) {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: s,
	})
}

func bitcoinStatus() {
	ch := make(chan DataSet)
	go func() {
		for {
			ch <- fetchData()
			time.Sleep(50 * time.Second)
		}
	}()
	var last *DataSet
	for {
		select {
		case f := <-ch:
			last = &f
		case <-time.After(time.Second / 3):
		}
		if last == nil {
			log.Printf("no data\n")
			continue
		}
		if last.Error != nil {
			log.Printf("error: %v\n", last.Error)
			setText(last.Error.Error())
		} else {
			t := time.Unix(int64(last.Latest.Time), 0)
			setText(fmt.Sprintf(
				"%s @ %s (%.0fm/%.0fs)",
				formatDollars(last.Price.Bitcoin["usd"]),
				formatInteger(last.Latest.Height),
				time.Since(t).Minutes(),
				time.Since(last.Fetched).Seconds(),
			))
		}
	}
}

func formatDollars(v float64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%.0f", math.Abs(v))
	if v < 0 {
		return "-$" + s
	} else {
		return "$" + s
	}
}

func formatInteger(v int) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d", v)
}

func getHeight() (*Latest, error) {
	return getJSONResource[Latest]("https://blockchain.info/latestblock")
}

func getPrice() (*Price, error) {
	return getJSONResource[Price]("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
}

func getJSONResource[T any](url string) (*T, error) {
	log.Printf("crawling %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status for %s: %s", url, resp.Status)
	}
	d := json.NewDecoder(resp.Body)
	var x T
	if err := d.Decode(&x); err != nil {
		return nil, err
	}
	return &x, nil

}
