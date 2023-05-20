package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Radictionary/website/pkg/config"
	"github.com/Radictionary/website/pkg/embedded_db"
	"github.com/Radictionary/website/pkg/models"
	"github.com/Radictionary/website/pkg/render"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type PacketStruct struct {
	Interface      string `json:"interface"`
	Protocol       string `json:"protocol"`
	SrcAddr        string `json:"srcAddr"`
	DstnAddr       string `json:"dstnAddr"`
	PacketNumber   int    `json:"packetNumber"`
	Time           string `json:"time"`
	Err            string `json:"err"`
	Customization  string `json:"customization"`
	ProtocolFilter string `json:"protocolFilter"`
}

type PacketRetrieval struct {
	PacketNumber int    `json:"packetNumber"`
	PacketDump   string `json:"packetDump"`
}

var (
	badgerDB    *embedded_db.DB
	stop        chan struct{}
	filterErr   bool
	packetInfo  PacketStruct
	handle      *pcap.Handle
	packetsInDB []string
	listening   bool
	dns         bool
	y           int = 1
)

func init() {
	go startSSE()
	listening = false
	messageChan = make(chan PacketStruct)
	stop = make(chan struct{})
	badgerDB, _ = embedded_db.NewDB("/tmp/badgerv4")
}

type RequestData struct {
	Protocols []string `json:"protocols"`
}

// Repo the repository used by the handlers
var Repo *Repository

// Repository is the repository type
type Repository struct {
	App *config.AppConfig
}

// NewRepo creates a new repository
func NewRepo(a *config.AppConfig) *Repository {
	return &Repository{
		App: a,
	}
}

// NewHandlers sets the repository for the handlers
func NewHandlers(r *Repository) {
	Repo = r
}

// detectProtocol detects the protocol and returns what it is
func detectProtocol(packet gopacket.Packet) (string, error) {
	// Check for transport layer
	if transport := packet.TransportLayer(); transport != nil {
		switch transport.LayerType() {
		case layers.LayerTypeTCP:
			return "TCP", nil
		case layers.LayerTypeUDP:
			// Check for DNS protocol
			if app := packet.ApplicationLayer(); app != nil {
				dnsLayer := &layers.DNS{}
				if err := dnsLayer.DecodeFromBytes(app.Payload(), gopacket.NilDecodeFeedback); err == nil {
					return "DNS", nil
				}
			}
			return "UDP", nil
		}
	}

	// Check for network layer
	if network := packet.NetworkLayer(); network != nil {
		switch network.LayerType() {
		case layers.LayerTypeIPv4:
			return "IPv4", nil
		case layers.LayerTypeIPv6:
			return "IPv6", nil
		case layers.LayerTypeICMPv4:
			return "ICMPv4", nil
		case layers.LayerTypeICMPv6:
			return "ICMPv6", nil
		}
	}

	// Check for ARP layer
	if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
		return "ARP", nil
	}

	return "N/A", errors.New("protocol not found")
}

// listenPackets function listens for packets in the background and sends packets to the frontend via SSE
func listenPackets() {
	fmt.Println("Started the goroutine")
	listening = true
	iface, err := badgerDB.Search("iface")
	config.Handle(err, "searching the database for iface", false)
	filter, err := badgerDB.Search("filter")
	config.Handle(err, "searching the database for iface", false)
	if strings.Contains(filter, " or dns or ") || strings.Contains(filter, "dns") || strings.Contains(filter, "dns or ") {
		filter = strings.Replace(filter, " or dns or ", "udp", -1)
		filter = strings.Replace(filter, "dns", "udp", -1)
		filter = strings.Replace(filter, "dns or ", "udp", -1)
		dns = true
	} else {
		dns = false
	}

	fmt.Println("Starting the goroutine with iface var being: ", iface)
	fmt.Println("Starting the goroutine with filter var being: ", filter)
	var (
		snaplen  = int32(1600)
		promisc  = false
		timeout  = pcap.BlockForever
		devFound = false
	)
	if iface == "" {
		badgerDB.Update("iface", "en0")
		iface = "en0"
		fmt.Println("Setting iface for the very first time to en0")
	}
	devices, err := pcap.FindAllDevs()
	if err != nil {
		config.Handle(err, "Finding all devices", true)
	}

	for _, device := range devices {
		if device.Name == iface {
			devFound = true
		}
	}
	if !devFound {
		config.Handle(err, "Device selected does not exist", true)
	}
	handle, err = pcap.OpenLive(iface, snaplen, promisc, timeout)
	config.Handle(err, "Finding all devices", true)

	if err := handle.SetBPFFilter(filter); err != nil {
		fmt.Println("Couldn't filter with current settings. Reseting the filter to be nothing. The filter was: ", filter)
		badgerDB.Update("filter", "")

		config.Handle(err, "Updating the database to reset filter", false)
		filterErr = true
	}
	source := gopacket.NewPacketSource(handle, handle.LinkType()) //LinkType() is the decoder to use
	for packet := range source.Packets() {
		select {
		case <-stop:
			fmt.Println("STOPPED THE GOROUTINE")
			listening = false
			dns = false
			return
		default:
			protocol, err := detectProtocol(packet)
			if dns && protocol != "DNS" {
				fmt.Println("Found packet but it wasn't dns")
				return
			}

			config.Handle(err, "detecting protocol", false)
			fmt.Println("Packet: ", y)
			networkLayer := packet.NetworkLayer()
			if networkLayer != nil {
				srcAddr := networkLayer.NetworkFlow().Src().String()
				dstnAddr := networkLayer.NetworkFlow().Dst().String()
				packetInfo.SrcAddr = srcAddr
				packetInfo.DstnAddr = dstnAddr
			} else if protocol == "ARP" {
				arpLayer := packet.Layer(layers.LayerTypeARP)
				arpPacket, _ := arpLayer.(*layers.ARP)

				srcIP := arpPacket.SourceHwAddress //srcMAC
				dstnIP := arpPacket.DstHwAddress   //dstMAC
				_ = arpPacket.SourceProtAddress
				_ = arpPacket.DstProtAddress
				packetInfo.SrcAddr = string(srcIP)
				packetInfo.DstnAddr = string(dstnIP)
			} else {
				packetInfo.SrcAddr = "not found"
				packetInfo.DstnAddr = "not found"
			}
			if filterErr {
				packetInfo.Err = "Filter was invalid. Reset the filter."
			}
			packetInfo.Protocol = protocol
			packetInfo.PacketNumber = y
			time_method, err := badgerDB.Search("time_method")
			if err != nil {
				time_method = "packet_timestamp"
			}
			//config.Handle(err, "searching the database for time_method", false)
			if time_method == "packet_timestamp" {
				packetInfo.Time = packet.Metadata().Timestamp.Format("15:04:05")
				packetInfo.Customization = "timestamp"
			} else if time_method == "packet_proccessed_timestamp" {
				packetInfo.Time = time.Now().Format("15:04:05")
				packetInfo.Customization = "proccessed_timestamp"
			}
			packetInfo.Interface = iface
			packetInfo.ProtocolFilter = filter
			messageChan <- packetInfo
			stry := strconv.Itoa(y)
			badgerDB.Update(stry, packet.Dump())
			packetsInDB = append(packetsInDB, stry)
			y++
		}
	}
}

// Home is the handler for the home page
func (m *Repository) Home(w http.ResponseWriter, r *http.Request) {
	render.RenderTemplate(w, "home.html", &models.TemplateData{})
}

var (
	clients        = make(map[http.ResponseWriter]struct{})
	clientsMutex   sync.Mutex
	messageChan    = make(chan PacketStruct)
	registerChan   = make(chan http.ResponseWriter)
	unregisterChan = make(chan http.ResponseWriter)
)

func sendToClient(client http.ResponseWriter, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling event data to JSON: %v\n", err)
		unregisterChan <- client
		return
	}

	_, err = fmt.Fprintf(client, "event: %s\ndata: %s\n\n", event, jsonData)
	if err != nil {
		fmt.Printf("Error sending SSE to client: %v\n", err)
		unregisterChan <- client
	}

	flusher, ok := client.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}
func sendToAllClients(event string, data interface{}) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	for client := range clients {
		sendToClient(client, event, data)
	}
}
func registerClient(w http.ResponseWriter) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	clients[w] = struct{}{}
}
func unregisterClient(w http.ResponseWriter) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	delete(clients, w)
}
func (m *Repository) SseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Expires", "0")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	registerChan <- w
	defer func() {
		unregisterChan <- w
	}()

	for packet := range messageChan {
		sendToAllClients("new-packet", packet)
		flusher.Flush()
	}
}
func startSSE() {
	for {
		select {
		case client := <-registerChan:
			registerClient(client)
		case client := <-unregisterChan:
			unregisterClient(client)
		}
	}
}

// InterfaceChange takes care of any changes to how to listen for packets
func (m *Repository) InterfaceChange(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/interface" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	switch r.Method {
	case "POST":
		if err := r.ParseForm(); err != nil {
			fmt.Fprintf(w, "ParseForm() err: %v", err)
			return
		}
		newiface := r.FormValue("interface")
		newfilter := r.Form["filter"]
		var newfilterstring string
		for i, filter := range newfilter {
			if i != 0 {
				newfilterstring += " or "
			}
			newfilterstring += filter
		}
		if newfilterstring != "" {
			fmt.Println("THE NEW FILTER IS: ", newfilterstring)
		}
		newTimeMethod := r.FormValue("time_method")
		body, err := io.ReadAll(r.Body)
		config.Handle(err, "Reading the body for new interface", false)

		if strings.Contains(string(body), "stop") {
			go func() {
				stop <- struct{}{}
				handle.Close()
				fmt.Println("Handle is closed")
			}()
			return
		} else if strings.Contains(string(body), "start") {
			if listening {
				handle.Close()
				fmt.Println("Closed the handle before starting")
			}
			go listenPackets()
		} else {
			go func() {
				if listening {
					handle.Close()
					fmt.Println("handle is closed")
				}
				badgerDB.Update("iface", newiface)
				badgerDB.Update("filter", newfilterstring)

				if newTimeMethod == "on" {
					badgerDB.Update("time_method", "packet_timestamp")
				} else {
					badgerDB.Update("time_method", "packet_proccessed_timestamp")
				}
				fmt.Println("Updated the embedded db")
				//y = 1
				if newfilterstring == "none" {
					newfilterstring = ""
				}
				if listening {
					go listenPackets()
				}
			}()
		}
	default:
		fmt.Fprintf(w, "Sorry, only POST methods are supported.")
		fmt.Println("NOT POST")
	}
	render.RenderTemplate(w, "interfacechange.html", &models.TemplateData{})
}

// SearchPackets retrieves packetDump about a packet that is stored in embedded database
func (m *Repository) SearchPacket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Println("Got the request to retrieve packet")
	packetNumber := r.URL.Query().Get("packetnumber")
	if packetNumber == "clear" {
		for _, value := range packetsInDB {
			err := badgerDB.Delete(value)
			if err != nil {
				return
			}
		}
		fmt.Println("CLEARED all of packetsInDB:", packetsInDB)
		y = 1
		packetsInDB = []string{} //reset it
	} else if packetNumber == "list" {
		badgerDB.View()
	} else if packetNumber == "sync"{
		
	} else {
		packetInfo, err := badgerDB.Search(packetNumber)
		config.Handle(err, "Searching DB for packetDump", false)
		if err != nil {
			fmt.Println("Packet not stored")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		packetNumberInt, err := strconv.Atoi(packetNumber)
		config.Handle(err, "Converting string to int", false)
		packetDump := PacketRetrieval{
			PacketNumber: packetNumberInt,
			PacketDump:   packetInfo,
		}
		// Marshal the packet object to JSON
		responseJSON, err := json.Marshal(packetDump)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(responseJSON)
		fmt.Println("Successfully found packet information and it is sent")
	}
}

