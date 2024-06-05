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

type Latest struct {
	Hash   string
	Height int
	Time   int
}

type Price struct {
	Bitcoin map[string]float64
}

type Data struct {
	Latest
	Price
}

func fetch() (*Data, error) {
	h, err := height()
	if err != nil {
		return nil, err
	}
	p, err := price()
	if err != nil {
		return nil, err
	}
	return &Data{*h, *p}, nil
}

func get[T any](u string) (*T, error) {
	log.Printf("crawling %s\n", u)
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	d := json.NewDecoder(resp.Body)
	var x T
	if err := d.Decode(&x); err != nil {
		return nil, err
	}
	return &x, nil

}

func height() (*Latest, error) {
	return get[Latest]("https://blockchain.info/latestblock")
}

func price() (*Price, error) {
	return get[Price]("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
}

func set(s string) {
	menuet.App().SetMenuState(&menuet.MenuState{
		Title: s,
	})
}

func helloClock() {
	ch := make(chan *Data)
	go func() {
		for {
			f, err := fetch()
			if err != nil {
				set(fmt.Errorf("can't fetch: %v", err).Error())
			} else {
				ch <- f
			}
			time.Sleep(time.Minute)
		}
	}()
	var last *Data
	for {
		select {
		case f := <-ch:
			last = f
		case <-time.After(time.Second):
		}
		if last == nil {
			continue
		}
		t := time.Unix(int64(last.Latest.Time), 0)
		set(fmt.Sprintf(
			"%s @ %s (%.0fm)",
			dollars(last.Price.Bitcoin["usd"]),
			integer(last.Latest.Height),
			time.Since(t).Minutes(),
		))
	}
}

func dollars(v float64) string {
	p := message.NewPrinter(language.English)
	s := p.Sprintf("%.0f", math.Abs(v))
	if v < 0 {
		return "-$" + s
	} else {
		return "$" + s
	}
}

func integer(v int) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d", v)
}

func main() {
	go helloClock()
	menuet.App().RunApplication()
}
