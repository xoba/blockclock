package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/caseymrm/menuet"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func main() {
	go bitcoinStatus()
	menuet.App().RunApplication()
}

func bitcoinStatus() {
	ch := make(chan DataSet)
	go func() {
		for {
			d := fetchData()
			dt := time.Minute
			const (
				max = 15 * time.Minute
				min = time.Minute
			)
			if d.Error == nil && d.BlockchainStats != nil {
				dt = time.Since(d.BlockchainStats.Time) / 2
				switch {
				case dt < 0:
					dt = time.Minute
				case dt > max:
					dt = max
				}
			}
			ch <- d
			if dt < min {
				dt = min
			}
			log.Printf("sleeping for %v\n", dt)
			time.Sleep(dt)
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
			continue
		}
		switch {
		case last.PriceInfo != nil && last.BlockchainStats != nil:
			setMenuTitle(fmt.Sprintf(
				"%s @ %s (%.0fm/%.0fs)",
				formatDollars(last.PriceInfo.Bitcoin["usd"]),
				formatInteger(last.BlockchainStats.Height),
				time.Since(last.BlockchainStats.Time).Minutes(),
				time.Since(last.Fetched).Seconds(),
			))
		case last.PriceInfo != nil:
			setMenuTitle(fmt.Sprintf(
				"%s (%.0fs)",
				formatDollars(last.PriceInfo.Bitcoin["usd"]),
				time.Since(last.Fetched).Seconds(),
			))
		case last.BlockchainStats != nil:
			setMenuTitle(fmt.Sprintf(
				"@ %s (%.0fm/%.0fs)",
				formatInteger(last.BlockchainStats.Height),
				time.Since(last.BlockchainStats.Time).Minutes(),
				time.Since(last.Fetched).Seconds(),
			))
		}
		if last.Error != nil {
			log.Printf("error: %v\n", last.Error)
			last.Error = nil
		}
	}
}

type DataSet struct {
	Error   error
	Fetched time.Time
	*BlockchainStats
	*PriceInfo
}

type BlockchainStats struct {
	Hash   string
	Height int
	Time   time.Time
}

type PriceInfo struct {
	Bitcoin map[string]float64
}

func fetchData() (out DataSet) {
	out.Fetched = time.Now()
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		h, err := getLatest()
		if err != nil {
			errs <- err
			return
		}
		out.BlockchainStats = h
	}()
	go func() {
		defer wg.Done()
		p, err := getPrice()
		if err != nil {
			errs <- err
			return
		}
		out.PriceInfo = p
	}()
	wg.Wait()
	select {
	case e := <-errs:
		out.Error = e
	default:
	}
	return
}

func setMenuTitle(title string) {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: title,
	})
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

func getLatest() (*BlockchainStats, error) {
	return getJSONResource[BlockchainStats]("https://api.blockcypher.com/v1/btc/main")
}

func getPrice() (*PriceInfo, error) {
	return getJSONResource[PriceInfo]("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
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
	buf, _ := json.Marshal(x)
	log.Printf("response: %s\n", string(buf))
	return &x, nil
}
