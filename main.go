package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caseymrm/menuet"
)

type Latest struct {
	Hash   string
	Height int
	Time   int
}

type Price struct {
	Bitcoin map[string]float64
}

func get[T any](u string) (*T, error) {
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
	for {
		if err := func() error {
			h, err := height()
			if err != nil {
				return err
			}
			p, err := price()
			if err != nil {
				return err
			}
			set(fmt.Sprintf("$%.2f @ %d", p.Bitcoin["usd"], h.Height))
			return nil
		}(); err != nil {
			set(fmt.Sprintf("error: %v", err))
		}
		time.Sleep(time.Minute)
	}
}

func main() {
	go helloClock()
	menuet.App().RunApplication()
}
