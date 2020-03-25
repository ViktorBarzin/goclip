package comms

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/viktorbarzin/goclip/clipboard"
	"github.com/viktorbarzin/goclip/common"
)

// Message used to (de)serialize messages
type Message struct {
	Content string
	Length  int
	Type    clipboard.ClipboardType
}

// MulticastClipboard contents on `interfacesNames` interfaces.
// Tries to get a UDP multicast address for each and multicasts on it.
func MulticastClipboard(addr string, interfacesNames []string) {
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
		if msg.Length > common.MaxDatagramSize {
			if !tcpListenerStarted {
				log.Println("Message size >", common.MaxDatagramSize, ". Falling back to peer-to-peer connection")
				log.Println("Starting TCP listener on port", common.PeerToPeerListenPort)
				go startTCPListener(common.PeerToPeerListenPort, msg)
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
	if len(interfacesNames) == 0 || common.Contains(interfacesNames, "all") {
		interfacesNames = common.Remove(interfacesNames, common.AllInterfaces)
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

// MsgHandler handles incoming messages and updates the local clipboard.
// If message size exceeds the single-packet size, a TCP connection is initialized to transfer to contents that way.
func MsgHandler(src *net.UDPAddr, n int, b []byte) {
	var msg Message
	json.Unmarshal(b[:n], &msg)
	log.Println("Read", strconv.Itoa(n), "bytes from broadcast traffic from", src)

	// If length > max UDP packet size, do peer-to-peer connection to get value
	if msg.Length > common.MaxDatagramSize {
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
	serverAddress := fmt.Sprintf(broadcasterAddress.String() + ":" + strconv.Itoa(common.PeerToPeerListenPort))
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

	conn.SetReadBuffer(common.MaxDatagramSize)

	// Loop forever reading from the socket
	buffer := make([]byte, common.MaxDatagramSize)
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
