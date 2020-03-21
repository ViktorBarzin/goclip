package main

import (
	"encoding/hex"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/atotto/clipboard"
	"github.com/urfave/cli"
)

const (
	defaultMulticastAddress = "239.0.0.0:9999"
	defaultRunTimeout       = 60
	maxDatagramSize         = 8192
)

func main() {
	app := cli.NewApp()

	app.Description = "kek"
	app.Name = "goclip"
	app.Usage = "Multicast clipboard contents over the network"
	app.Commands = []cli.Command{
		{
			Name:  "send",
			Usage: "send clipboard contents",
			Action: func(c *cli.Context) error {
				return sendHandler(c)
			},
		},
		{
			Name:  "receive",
			Usage: "receive clipboard contents",
			Action: func(c *cli.Context) error {
				return receiveHandler(c)
			},
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "timeout, t",
			Usage: "Seconds for which the application will be performing the action (send, receive). After this exit.",
			Value: strconv.Itoa(defaultRunTimeout),
		},
	}

	// app.Action = func(c *cli.Context) error {
	// 	address := c.Args().Get(0)
	// 	if address == "" {
	// 		address = defaultMulticastAddress
	// 	}
	// 	go multicast.Listen(address, msgHandler)
	// 	multicastClipboard(address)
	// 	return nil
	// }

	app.Run(os.Args)
}

func waitTimeout(durationStr string) <-chan time.Time {
	durationInt := defaultRunTimeout
	if _, err := strconv.Atoi(durationStr); err == nil {
		// log.Fatal("Duration must be a number. Got:", durationStr, "Using default:", defaultRunTimeout)
		durationInt, _ = strconv.Atoi(durationStr)
	}
	return time.After(time.Duration(durationInt) * time.Second)
}

func receiveHandler(c *cli.Context) error {
	quit := make(chan string, 1)
	go func() {
		Listen(defaultMulticastAddress, msgHandler)
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

func sendHandler(c *cli.Context) error {
	go func() { multicastClipboard(defaultMulticastAddress) }()
	durationStr := c.String("timeout")
	print(c.Args())
	<-waitTimeout(durationStr)
	return nil
}

func multicastClipboard(addr string) {
	conn, err := NewBroadcaster(addr)
	if err != nil {
		log.Fatal(err)
	}

	for {
		clip, _ := clipboard.ReadAll()
		log.Println("Multicasting clipboard contents to", addr, "\n", clip)
		conn.Write(encode(clip))
		time.Sleep(1 * time.Second)
	}
}

func msgHandler(src *net.UDPAddr, n int, b []byte) {
	decodedStr := decode(b[:n])
	log.Println(n, "bytes read from", src, "Inseting into clipboard:\n", decodedStr)
	clipboard.WriteAll(decodedStr)
}

func decode(src []byte) string {
	dst := make([]byte, hex.DecodedLen(len(src)))
	n, err := hex.Decode(dst, src)
	if err != nil {
		log.Fatal(err)
	}
	return string(dst[:n])
}

func encode(str string) []byte {
	src := []byte(str)

	dst := make([]byte, hex.EncodedLen(len(src)))
	hex.Encode(dst, src)

	return dst
}

// Listen binds to the UDP address and port given and writes packets received
// from that address to a buffer which is passed to a hander
func Listen(address string, handler func(*net.UDPAddr, int, []byte)) {
	// Parse the string address
	addr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		log.Fatal(err)
	}

	// Open up a connection
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Fatal(err)
	}

	conn.SetReadBuffer(maxDatagramSize)

	// Loop forever reading from the socket
	buffer := make([]byte, maxDatagramSize)
	numBytes, src, err := conn.ReadFromUDP(buffer)
	if err != nil {
		log.Fatal("ReadFromUDP failed:", err)
	}

	handler(src, numBytes, buffer)
}

// NewBroadcaster creates a new UDP multicast connection on which to broadcast
func NewBroadcaster(address string) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
