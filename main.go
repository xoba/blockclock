package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/caseymrm/menuet"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

func main() {

	runws()

	go bitcoinStatus()

	menuet.App().RunApplication()
}

func runws() {

	ntfnHandlers := rpcclient.NotificationHandlers{
		OnFilteredBlockConnected: func(height int32, header *wire.BlockHeader, txns []*btcutil.Tx) {
			log.Printf("Block connected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
		OnFilteredBlockDisconnected: func(height int32, header *wire.BlockHeader) {
			log.Printf("Block disconnected: %v (%d) %v",
				header.BlockHash(), height, header.Timestamp)
		},
	}

	connCfg := &rpcclient.ConnConfig{
		Host:     "localhost:8334",
		Endpoint: "ws",
		User:     os.Getenv("user"),
		Pass:     os.Getenv("pass"),
	}
	client, err := rpcclient.New(connCfg, &ntfnHandlers)
	if err != nil {
		log.Fatalf("can't connect: %v", err)
	}

	// Register for block connect and disconnect notifications.
	if err := client.NotifyBlocks(); err != nil {
		log.Fatal(err)
	}
	log.Println("NotifyBlocks: Registration Complete")

	// Get the current block count.
	blockCount, err := client.GetBlockCount()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Block count: %d", blockCount)

	// For this example gracefully shutdown the client after 10 seconds.
	// Ordinarily when to shutdown the client is highly application
	// specific.
	log.Println("Client shutdown in 10 seconds...")
	time.AfterFunc(time.Second*10, func() {
		log.Println("Client shutting down...")
		client.Shutdown()
		log.Println("Client shutdown complete.")
	})

	// Wait until the client either shuts down gracefully (or the user
	// terminates the process with Ctrl+C).
	client.WaitForShutdown()
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
		h, err := getStats()
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

func getStats() (*BlockchainStats, error) {
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
