package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/MatejLach/dynafire/firewall/firewalld"
	"github.com/MatejLach/dynafire/provider/turris"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	fwc, err := firewalld.New()
	if err != nil {
		slog.Error("Initialization failed; host system pre-requisites not met", "details", err)
		os.Exit(1)
	}

	tc, err := turris.NewClient(turris.Url, turris.Port)
	if err != nil {
		slog.Error("Unable to initialize Turris dynafire client", "details", err)
		os.Exit(1)
	}

	err = tc.Connect()
	if err != nil {
		slog.Error("Unable to connect to Turris firewall update server", "details", err)
		os.Exit(1)
	}

	wg := sync.WaitGroup{}
	sem := make(chan struct{}, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		tc.RequestMessages(context.Background())
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for listMsg := range tc.ListChan {
			slog.Info(fmt.Sprintf("adding %d IPs to the blacklist", len(listMsg.Blacklist)))
			sem <- struct{}{}

			err = fwc.ResetFirewallRules()
			if err != nil {
				slog.Error("unable to clear old IP blacklist", "details", err)
				os.Exit(1)
			}

			err = fwc.BlockIPList(listMsg.Blacklist)
			if err != nil {
				slog.Error("unable to initialize IP blacklist", "details", err)
				os.Exit(1)
			}
			slog.Info("Starting to process delta updates...")
			<-sem
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for deltaMsg := range tc.DeltaChan {
			sem <- struct{}{}

			// 'positive' operation adds an IP to the blacklist
			// 'negative' removes an existing IP from the blacklist
			switch deltaMsg.Operation {
			case "positive":
				err = fwc.BlockIP(deltaMsg.IP)
				if err != nil {
					slog.Error("unable to blacklist IP", "details", err)
					os.Exit(1)
				}

				slog.Debug("blacklisting", "IP", deltaMsg.IP.String())
			case "negative":
				err = fwc.UnblockIP(deltaMsg.IP)
				if err != nil {
					slog.Error("unable to whitelist IP", "details", err)
					os.Exit(1)
				}

				slog.Debug("whitelisting", "IP", deltaMsg.IP.String())
			}
			time.Sleep(250 * time.Millisecond)
			<-sem
		}
	}()

	wg.Wait()
}
