package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/urfave/cli"
	"github.com/viktorbarzin/goclip/clipboard"
)

const (
	defaultMulticastAddress = "239.0.0.0:9999"
	defaultRunTimeout       = 60
	maxDatagramSize         = 8192
	peerToPeerListenPort    = 30099
	allInterfaces           = "all"
)

// Message used to (de)serialize messages
type Message struct {
	Content string
	Length  int
	Type    clipboard.ClipboardType
}

func main() {
	app := cli.NewApp()

	app.Description = fmt.Sprint("Application to share your clipboard over a LAN. The content is multicasted to ", defaultMulticastAddress, ". If the content exceeds the maximum UDP datagram size of ", maxDatagramSize, " bytes then peer-to-peer TCP connection is initialized and content is send over it instead.")
	app.Name = "goclip"
	app.Usage = "Multicast clipboard contents over the network"
	app.Commands = []cli.Command{
		{
			Name:  "send",
			Usage: "send clipboard contents",
			Action: func(c *cli.Context) error {
				return sendHandler(c)
			},
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "timeout, t",
					Usage: "Seconds for which the application will be performing the action (send, receive). After this exit.",
					Value: defaultRunTimeout,
				},
				cli.StringSliceFlag{
					Name:  "interface, i",
					Usage: "Interface to multicast on. Can be specified multiple times. (default: \"all\")",
					// Value: &cli.StringSlice{allInterfaces},  // Does not work as default value, but appends
				},
			},
		},
		{
			Name:  "receive",
			Usage: "receive clipboard contents",
			Action: func(c *cli.Context) error {
				return receiveHandler(c)
			},
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "timeout, t",
					Usage: "Seconds for which the application will be performing the action (send, receive). After this exit.",
					Value: defaultRunTimeout,
				},
			},
		},
	}

	app.Run(os.Args)
}

func waitTimeout(durationStr string) <-chan time.Time {
	durationInt, err := strconv.Atoi(durationStr)
	if err != nil {
		log.Println("Duration must be a number. Got:", durationStr, "Using default:", defaultRunTimeout)
		durationInt = defaultRunTimeout
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
	quit := make(chan string, 1)
	durationStr := c.String("timeout")
	go func() {
		interfaces := c.StringSlice("interface")
		multicastClipboard(defaultMulticastAddress, interfaces)
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

func multicastClipboard(addr string, interfacesNames []string) {
	multicasters, broadcastingInterfaces := getMulticasters(addr, interfacesNames)
	if len(multicasters) == 0 {
		log.Fatalln("Could not find any interfaces to multicast on. Available interfaces:", getAllInterfaceNames())
	}

	tcpListenerStarted := false

	for {
		clip, clipType, _ := clipboard.GetEncodedClipboard()
		msg := Message{Content: clip, Type: clipType, Length: len(clip)}
		log.Println("Multicasting", msg.Length, " bytes of clipboard contents to", addr, "on interfaces:", broadcastingInterfaces)
		// If msg len is bigger than the UDP datagram, do a TCP peer-to-peer connection
		if msg.Length > maxDatagramSize {
			if !tcpListenerStarted {
				log.Println("Message size >", maxDatagramSize, ". Falling back to peer-to-peer connection")
				log.Println("Starting TCP listener on port", peerToPeerListenPort)
				go startTCPListener(peerToPeerListenPort, msg)
				tcpListenerStarted = true
			}
			msg.Content = "" // Do not send content here
		}
		encoded, _ := json.Marshal(msg)
		multicastMessage(multicasters, encoded)
		time.Sleep(1 * time.Second)
	}
}

func getMulticasters(addr string, interfacesNames []string) ([]*net.UDPConn, []string) {
	var multicasters []*net.UDPConn
	var broadcastingInterfaces []string
	sort.Strings(interfacesNames)

	// if empty interfaces, use all
	if len(interfacesNames) == 0 || contains(interfacesNames, "all") {
		interfacesNames = remove(interfacesNames, allInterfaces)
		log.Println("Trying to get multicast address for all interfaces")
		interfacesNames = getAllInterfaceNames()
	}

	for _, interfaceName := range interfacesNames {
		b, err := NewBroadcaster(addr, interfaceName)
		if err != nil {
			log.Println("Error for interface", interfaceName, ":", err.Error()+".", "Skipping...")
			continue
		}
		multicasters = append(multicasters, b)
		broadcastingInterfaces = append(broadcastingInterfaces, interfaceName)
	}
	return multicasters, broadcastingInterfaces
}

func getAllInterfaceNames() []string {
	var interfacesNames []string
	addrs, _ := net.Interfaces()
	for _, el := range addrs {
		interfacesNames = append(interfacesNames, el.Name)
	}
	return interfacesNames
}

func contains(s []string, searchterm string) bool {
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}
func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func multicastMessage(multicasters []*net.UDPConn, encoded []byte) {
	for _, multicaster := range multicasters {
		go multicaster.Write(encoded)
	}
}

func startTCPListener(listenPort int, messageToSend Message) {
	addr, err := net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(listenPort))
	if err != nil {
		log.Fatalln(err)
	}

	conn, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	for {
		client, _ := conn.Accept()
		go func(connection net.Conn) {
			defer client.Close()
			log.Println("Sending", messageToSend.Length, "bytes to", client.RemoteAddr())
			encoded, _ := json.Marshal(messageToSend)
			client.Write(encoded)
			client.Write([]byte("\n")) // Delimiter
		}(client)
	}

}
func msgHandler(src *net.UDPAddr, n int, b []byte) {
	var msg Message
	json.Unmarshal(b[:n], &msg)
	log.Println("Read", strconv.Itoa(n), "bytes from broadcast traffic from", src)

	// If length > max UDP packet size, do peer-to-peer connection to get value
	if msg.Length > maxDatagramSize {
		decodedMessage, err := getClipboardFromPeer(src.IP, msg.Length)
		if err != nil {
			log.Fatal(err)
			return
		}
		msg.Content = decodedMessage.Content
	}
	clipboard.StoreClipboard(msg.Content, msg.Type)
}

func getClipboardFromPeer(broadcasterAddress net.IP, contentLength int) (Message, error) {
	serverAddress := fmt.Sprintf(broadcasterAddress.String() + ":" + strconv.Itoa(peerToPeerListenPort))
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		return Message{}, err
	}
	defer conn.Close()

	clientResponse, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return Message{}, err
	}
	var decodedMessage Message
	err = json.Unmarshal(clientResponse, &decodedMessage)
	if err != nil {
		return Message{}, fmt.Errorf("Could not decode peer response: " + err.Error())
	}
	log.Println("Got", decodedMessage.Length, "bytes of content from direct peer-to-peer traffic with", serverAddress)
	return decodedMessage, nil
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
func NewBroadcaster(address string, interfaceName string) (*net.UDPConn, error) {
	remoteAddr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return nil, err
	}

	localIP, err := getIPForInterface(interfaceName)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp4", &net.UDPAddr{IP: localIP}, remoteAddr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func getIPForInterface(interfaceName string) (net.IP, error) {
	ief, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return net.IP{}, err
	}
	addrs, err := ief.Addrs()
	if err != nil {
		return net.IP{}, err
	}

	if len(addrs) == 0 {
		return net.IP{}, errors.New("No address found for this interface")
	}
	addr := &net.UDPAddr{IP: addrs[0].(*net.IPNet).IP}
	return addr.IP, nil
}
