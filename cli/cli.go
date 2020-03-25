package cli

import (
	"fmt"
	"log"
	"strconv"
	"time"

	urfaveCLI "github.com/urfave/cli"
	"github.com/viktorbarzin/goclip/common"
	"github.com/viktorbarzin/goclip/comms"
)

// GetApp return the CLI application
func GetApp() *urfaveCLI.App {
	app := urfaveCLI.NewApp()

	app.Description = fmt.Sprint("Application to share your clipboard over a LAN. The content is multicasted to ", common.DefaultMulticastAddress, ". If the content exceeds the maximum UDP datagram size of ", common.MaxDatagramSize, " bytes then peer-to-peer TCP connection is initialized and content is send over it instead.")
	app.Name = "goclip"
	app.Usage = "Multicast clipboard contents over the network"
	app.Commands = []urfaveCLI.Command{
		{
			Name:  "send",
			Usage: "send clipboard contents",
			Action: func(c *urfaveCLI.Context) error {
				return sendHandler(c)
			},
			Flags: []urfaveCLI.Flag{
				urfaveCLI.IntFlag{
					Name:  "timeout, t",
					Usage: "Seconds for which the application will be performing the action (send, receive). After this exit.",
					Value: common.DefaultRunTimeout,
				},
				urfaveCLI.StringSliceFlag{
					Name:  "interface, i",
					Usage: "Interface to multicast on. Can be specified multiple times. (default: \"all\")",
					// Value: &urfaceCLI.StringSlice{allInterfaces},  // Does not work as default value, but appends
				},
			},
		},
		{
			Name:  "receive",
			Usage: "receive clipboard contents",
			Action: func(c *urfaveCLI.Context) error {
				return receiveHandler(c)
			},
			Flags: []urfaveCLI.Flag{
				urfaveCLI.IntFlag{
					Name:  "timeout, t",
					Usage: "Seconds for which the application will be performing the action (send, receive). After this exit.",
					Value: common.DefaultRunTimeout,
				},
			},
		},
	}
	return app
}

func receiveHandler(c *urfaveCLI.Context) error {
	quit := make(chan string, 1)
	go func() {
		comms.Listen(common.DefaultMulticastAddress, comms.MsgHandler)
		quit <- "done"
	}()
	durationStr := c.String("timeout")
	log.Println("Waiting", durationStr, "seconds to receive clipboard contents")
	select {
	case <-quit:
		log.Println("Received clipboard contents. Closing receiver")
	case <-waitTimeout(durationStr):
		log.Println("Timed out without receiving anything.")
	}
	return nil
}
func sendHandler(c *urfaveCLI.Context) error {
	quit := make(chan string, 1)
	durationStr := c.String("timeout")
	go func() {
		interfaces := c.StringSlice("interface")
		comms.MulticastClipboard(common.DefaultMulticastAddress, interfaces)
		quit <- "done"
	}()

	select {
	case <-quit:
		log.Println("Done")
	case <-waitTimeout(durationStr):
		log.Println("Reached broadcasting limit of", durationStr, "seconds")
	}
	return nil
}

func waitTimeout(durationStr string) <-chan time.Time {
	durationInt, err := strconv.Atoi(durationStr)
	if err != nil {
		log.Println("Duration must be a number. Got:", durationStr, "Using default:", common.DefaultRunTimeout)
		durationInt = common.DefaultRunTimeout
	}
	return time.After(time.Duration(durationInt) * time.Second)
}
