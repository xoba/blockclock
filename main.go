package main

import (
	"encoding/json"
	"fmt"
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

type LatestBlockchain struct {
	Hash   string
	Height int
	Time   int
}

type CoinGeckoPrice struct {
	Bitcoin map[string]float64
}

type DataSet struct {
	LatestBlockchain
	CoinGeckoPrice
}

func fetchData() (*DataSet, error) {
	h, err := getHeight()
	if err != nil {
		return nil, err
	}
	p, err := getPrice()
	if err != nil {
		return nil, err
	}
	return &DataSet{*h, *p}, nil
}

func setText(s string) {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: s,
	})
}

func bitcoinStatus() {
	ch := make(chan *DataSet)
	go func() {
		for {
			f, err := fetchData()
			if err != nil {
				setText(err.Error())
			} else {
				ch <- f
			}
			time.Sleep(time.Minute)
		}
	}()
	var last *DataSet
	for {
		select {
		case f := <-ch:
			last = f
		case <-time.After(time.Second):
		}
		if last == nil {
			continue
		}
		t := time.Unix(int64(last.LatestBlockchain.Time), 0)
		setText(fmt.Sprintf(
			"%s @ %s (%.0fm)",
			formatDollars(last.CoinGeckoPrice.Bitcoin["usd"]),
			formatInteger(last.LatestBlockchain.Height),
			time.Since(t).Minutes(),
		))
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

func getHeight() (*LatestBlockchain, error) {
	return getJSONResource[LatestBlockchain]("https://blockchain.info/latestblock")
}

func getPrice() (*CoinGeckoPrice, error) {
	return getJSONResource[CoinGeckoPrice]("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
}

func getJSONResource[T any](url string) (*T, error) {
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
