package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	"github.com/google/gopacket/pcapgo"
)

type PacketStruct struct {
	Interface    string `json:"interface"`
	Protocol     string `json:"protocol"`
	SrcAddr      string `json:"srcAddr"`
	DstnAddr     string `json:"dstnAddr"`
	PacketNumber int    `json:"packetNumber"`
	Time         string `json:"time"`
	Err          string `json:"err"`
}
type PacketRetrieval struct {
	PacketNumber string `json:"packetNumber"`
	PacketDump   string `json:"packetDump"`
}
type SettingsRetrieval struct {
	Interface       string `json:"interface"`
	Filter          string `json:"filter"`
	TimeStampMethod string `json:"timeStampMethod"`
}

var (
	badgerDB          *embedded_db.DB = embedded_db.NewDB("/tmp/badgerv4")
	app               *config.AppConfig
	stop                   = make(chan struct{})
	filterErr         bool = false
	interfaceErr      bool = false
	packetInfo        PacketStruct
	settingsRetrieval SettingsRetrieval
	handle            *pcap.Handle
	packetsInDB       []string
	listening         bool = false
	y                 int  = 1
)

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
func detectProtocol(packet gopacket.Packet) (string, string, string) {
	var protocol, sourceAddress, destAddress string

	if transport := packet.TransportLayer(); transport != nil {
		switch transport.LayerType() {
		case layers.LayerTypeTCP:
			protocol = "TCP"
			tcp, _ := transport.(*layers.TCP)
			sourceAddress = packet.NetworkLayer().NetworkFlow().Src().String()
			destAddress = packet.NetworkLayer().NetworkFlow().Dst().String()
			if tcp.DstPort == 80 || tcp.SrcPort == 80 {
				protocol = "HTTP"
			}
			if tcp.DstPort == 443 || tcp.SrcPort == 443 {
				protocol = "HTTPS"
			}
		case layers.LayerTypeUDP:
			protocol = "UDP"
			sourceAddress = packet.NetworkLayer().NetworkFlow().Src().String()
			destAddress = packet.NetworkLayer().NetworkFlow().Dst().String()
		}
	}

	if protocol == "" {
		if network := packet.NetworkLayer(); network != nil {
			switch network.LayerType() {
			case layers.LayerTypeIPv4:
				protocol = "IPv4"
				ipv4, _ := network.(*layers.IPv4)
				sourceAddress = ipv4.SrcIP.String()
				destAddress = ipv4.DstIP.String()
			case layers.LayerTypeIPv6:
				protocol = "IPv6"
				ipv6, _ := network.(*layers.IPv6)
				sourceAddress = ipv6.SrcIP.String()
				destAddress = ipv6.DstIP.String()
			case layers.LayerTypeICMPv4:
				protocol = "ICMPv4"
			case layers.LayerTypeICMPv6:
				protocol = "ICMPv6"
			}
		}
	}

	if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
		protocol = "ARP"
		arpPacket := arpLayer.(*layers.ARP)
		sourceAddress = net.IP(arpPacket.SourceProtAddress).String()
		destAddress = net.IP(arpPacket.DstProtAddress).String()
	}

	if protocol == "" {
		protocol = "N/A"
	}

	return protocol, sourceAddress, destAddress
}

// listenPackets function listens for packets in the background and sends packets to the frontend via SSE
func listenPackets() {
	listening = true
	iface, err := badgerDB.Search("iface")
	config.Handle(err, "searching the database for iface", false)
	filter, err := badgerDB.Search("filter")
	config.Handle(err, "searching the database for filter", false)
	time_method, err := badgerDB.Search("time_method")
	config.Handle(err, "Getting the user set time method", false)
	file_save, err := badgerDB.Search("savePath")
	config.Handle(err, "Getting the user set save path", false)

	fmt.Printf("Starting the goroutine\tinterface:%v\tfilter:%v\ttime_method:%v\n", iface, filter, time_method)
	var (
		snaplen  = int32(1600)
		promisc  = false
		timeout  = pcap.BlockForever
		devFound = false
	)
	if iface == "" {
		badgerDB.Update("iface", "en0")
		iface = "en0"
		interfaceErr = true
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
	var pcapFile *os.File
	if file_save != "" {
		pcapFile, err = os.Create(file_save + ".pcap")
		if err != nil {
			config.Handle(err, "Creating pcap file", true)
		}
		defer pcapFile.Close()
	}
	pcapWriter := pcapgo.NewWriter(pcapFile)
	pcapWriter.WriteFileHeader(uint32(snaplen), handle.LinkType())

	for packet := range source.Packets() {
		select {
		case <-stop:
			fmt.Println("STOPPED THE GOROUTINE")
			listening = false
			return
		default:
			var protocol string
			protocol, packetInfo.SrcAddr, packetInfo.DstnAddr = detectProtocol(packet)
			fmt.Println("Packet: ", y)
			if filterErr {
				packetInfo.Err = "Filter was invalid. Reset the filter."
			} else if interfaceErr {
				packetInfo.Err = "Interface was invalid. Reset the interface to en0."
			}
			packetInfo.Protocol = protocol
			packetInfo.PacketNumber = y
			if time_method == "packet_proccessed_timestamp" {
				packetInfo.Time = time.Now().Format("15:04:01")
			} else {
				packetInfo.Time = packet.Metadata().Timestamp.Format("15:04:05")

			}
			packetInfo.Interface = iface
			messageChan <- packetInfo
			stry := strconv.Itoa(y)
			badgerDB.Update(stry, packet.Dump())    //must be stored as string because that is currently badgerDB implementation
			packetsInDB = append(packetsInDB, stry) //keeps track of the packets in the database to remove later if needed by the user

			if file_save != "" {
				err := pcapWriter.WritePacket(packet.Metadata().CaptureInfo, packet.Data())
				if err != nil {
					config.Handle(err, "Writing packet to pcap file", false)
				}
			}

			y++

		}
	}
}

// Home is the handler for the home page
func (m *Repository) Home(w http.ResponseWriter, r *http.Request) {
	go startSSE()
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
func (m *Repository) Change(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/change" {
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
		config.Handle(err, "Reading the body for new changes", false)
		fmt.Println("body is:", string(body))
		var data map[string]interface{}

		// Parse the JSON response
		if err := json.Unmarshal(body, &data); err != nil {
			fmt.Println("Error parsing JSON:", err)
			return
		}

		// Check if the "fullPath" key exists in the parsed data
		if fullPath, ok := data["fullPath"].(string); ok {
			badgerDB.Update("savePath", fullPath)
		} else {
			fmt.Println("Full path not found in JSON")
		}

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
		} else if strings.Contains(string(body), "reset") {
			if listening {
				handle.Close()
				fmt.Println("handle is closed")
			}
			y = 1
			go listenPackets()
		} else {
			go func() {
				if listening {
					handle.Close()
					fmt.Println("handle is closed")
				}
				if newiface != "" {
					badgerDB.Update("inteface", newiface)
				}
				if newfilterstring != "" && newfilterstring != "none" {
					badgerDB.Update("filter", newfilterstring)
				} else if newfilterstring == "none" {
					badgerDB.Update("filter", "")
				}
				if newTimeMethod == "on" {
					badgerDB.Update("time_method", "packet_timestamp")
				} else {
					badgerDB.Update("time_method", "packet_proccessed_timestamp")
				}
				fmt.Println("Updated the embedded db")
				if listening {
					go listenPackets() //don't stop listening for packets if it is already listening
				}
			}()
		}
	default:
		fmt.Fprintf(w, "Sorry, only POST methods are supported.")
		fmt.Println("NOT POST")
	}
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
	} else if packetNumber == "list" && !app.InProduction { //only have the database listing current data in development
		badgerDB.View()
	} else {
		packetInfo, err := badgerDB.Search(packetNumber)
		if err != nil {
			fmt.Println("Packet not stored")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		config.Handle(err, "Converting string to int", false)
		packetDump := PacketRetrieval{
			PacketNumber: packetNumber,
			PacketDump:   packetInfo,
		}

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

func (m *Repository) SettingsSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	iface, err := badgerDB.Search("iface")
	config.Handle(err, "searching the database for iface", false)
	settingsRetrieval.Interface = iface

	filter, err := badgerDB.Search("filter")
	config.Handle(err, "searching the database for iface", false)

	settingsRetrieval.Filter = filter

	time_method, err := badgerDB.Search("time_method")
	if err != nil {
		time_method = "packet_timestamp" //setting the timestamp method for the first time
	}
	if time_method == "packet_timestamp" {
		settingsRetrieval.TimeStampMethod = "timestamp"
	} else if time_method == "packet_proccessed_timestamp" {
		settingsRetrieval.TimeStampMethod = "proccessed_timestamp"
	}

	jsonPayload, err := json.Marshal(settingsRetrieval)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonPayload)
}
