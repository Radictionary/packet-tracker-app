package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Radictionary/website/pkg/config"
	"github.com/Radictionary/website/pkg/embedded_db"
	"github.com/Radictionary/website/pkg/models"
	"github.com/Radictionary/website/pkg/render"
	"github.com/dgraph-io/badger/v4"
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
	ProtocolFilter string `json:protocolFilter`
	PacketDump     string `json:"packetDump"`
}

type PacketRetrieval struct {
	PacketNumber int    `json:"packetNumber"`
	PacketDump   string `json:"packetDump"`
}

var (
	messageChan chan PacketStruct
	readyChan   chan string
	stop        chan struct{}
	stopped     chan bool
	filterErr   bool
	packetInfo  PacketStruct
	db          *badger.DB
	handle      *pcap.Handle
	packetsInDB []string
	listening   bool
)

func init() {
	messageChan = make(chan PacketStruct)
	readyChan = make(chan string)
	stop = make(chan struct{})
	stopped = make(chan bool, 100)

	var err error
	db, err = embedded_db.CallDatabase()
	config.Handle(err, "opening the embeded database", true)
}

type RequestData struct {
	Protocols []string `json:"protocols"`
}

var y int = 1

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
	var protocol string
	var err error
	// Check for transport layer
	if transport := packet.TransportLayer(); transport != nil {
		switch transport.LayerType() {
		case layers.LayerTypeTCP:
			protocol = "TCP"
		case layers.LayerTypeUDP:
			protocol = "UDP"
		default:
			protocol = ""
		}
	} else if network := packet.NetworkLayer(); network != nil {
		// Check for network layer
		switch network.LayerType() {
		case layers.LayerTypeIPv4:
			protocol = "IPv4"
		case layers.LayerTypeIPv6:
			protocol = "IPv6"
		case layers.LayerTypeICMPv4:
			protocol = "ICMPv4"
		case layers.LayerTypeICMPv6:
			protocol = "ICMPv6"
		default:
			protocol = ""
		}
	} else if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
		protocol = "ARP"
	}
	if protocol == "" {
		err = errors.New("protocol not found")
		protocol = "N/A"
	}
	return protocol, err
}

// listenPackets function listens for packets in the background and sends packets to the frontend via SSE
func listenPackets() {
	fmt.Println("Started the goroutine")
	listening = true
	iface, err := embedded_db.SearchDatabase(db, "iface")
	config.Handle(err, "searching the database for iface", false)
	filter, err := embedded_db.SearchDatabase(db, "filter")
	config.Handle(err, "searching the database for iface", false)

	fmt.Println("Starting the goroutine with iface var being: ", iface)
	fmt.Println("Starting the goroutine with filter var being: ", filter)
	var (
		snaplen  = int32(1600)
		promisc  = false
		timeout  = pcap.BlockForever
		devFound = false
	)
	if iface == "" {
		embedded_db.UpdateDatabase(db, "iface", "en0")
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

	// defer func() {
	// 	handle.Close()
	// 	fmt.Println("Closed the handle")
	// }()

	if err := handle.SetBPFFilter(filter); err != nil {
		fmt.Println("Couldn't filter with current settings. Reseting the filter to be nothing. The filter was: ", filter)
		err := embedded_db.UpdateDatabase(db, "filter", "")
		config.Handle(err, "Updating the database to reset filter", false)
		filterErr = true
	}
	source := gopacket.NewPacketSource(handle, handle.LinkType()) //LinkType() is the decoder to use
	for packet := range source.Packets() {
		select {
		case <-stop:
			fmt.Println("STOPPED THE GOROUTINE")
			stopped <- true
			listening = false
			return
		default:
			protocol, err := detectProtocol(packet)
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
			time_method, err := embedded_db.SearchDatabase(db, "time_method")
			config.Handle(err, "searching the database for time_method", false)
			if time_method == "packet_timestamp" {
				packetInfo.Time = packet.Metadata().Timestamp.Format("15:04:05")
				packetInfo.Customization = "timestamp"
			} else if time_method == "packet_proccessed_timestamp" {
				packetInfo.Time = time.Now().Format("15:04:05")
				packetInfo.Customization = "proccessed_timestamp"
			}
			packetInfo.Interface = iface
			packetInfo.ProtocolFilter = filter
			packetInfo.PacketDump = packet.Dump()
			messageChan <- packetInfo
			stry := strconv.Itoa(y)
			embedded_db.UpdateDatabase(db, stry, packetInfo.PacketDump)
			packetsInDB = append(packetsInDB, stry)
			fmt.Println("Updated the database with: ", stry)
			y++
		}
	}
}

// Home is the handler for the home page
func (m *Repository) Home(w http.ResponseWriter, r *http.Request) {
	render.RenderTemplate(w, "home.html", &models.TemplateData{})
}

func formatServerSentEvent(event string, data any) (string, error) {
	m := map[string]any{
		"data": data,
	}
	buff := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buff)
	err := encoder.Encode(m)
	if err != nil {
		return "", err
	}
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("event: %s\n", event))
	sb.WriteString(fmt.Sprintf("data: %v\n\n", buff.String()))
	return sb.String(), nil
}

// Sends packets to the frontend
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
	fmt.Println("Request received from ", r.RemoteAddr)

	for packet := range messageChan {
		jsonData, err := json.Marshal(packet)
		if err != nil {
			panic(err)
		}
		event, err := formatServerSentEvent("new-packet-update", string(jsonData))
		config.Handle(err, "Error formatting server sent event", true)

		_, err = fmt.Fprint(w, event)
		config.Handle(err, "Error sending to client", false)

		flusher.Flush()
		fmt.Printf("Flushed the data\n")
	}
}

// ReadySSE function takes care of changes to settings, and notifies frontend when program is ready via SSE
func (m *Repository) ReadySSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	for ready := range readyChan {
		event, err := formatServerSentEvent("ready-update", ready)
		config.Handle(err, "Error formatting server sent event", true)
		_, err = fmt.Fprint(w, event)
		config.Handle(err, "Error sending to client", true)
		flusher.Flush()
		fmt.Printf("Sent the signal to redirect the user\n")
	}
}

// InterfaceChange takes care of any changes to how to listen for packets
func (m *Repository) InterfaceChange(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Recieved the POST request to change the interface")
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
		fmt.Println("THE NEW FILTER IS:", newfilter)
		for i, filter := range newfilter {
			if i != 0 {
				newfilterstring += " or "
			}
			newfilterstring += filter
		}
		fmt.Println(newfilterstring)
		newTimeMethod := r.FormValue("time_method")
		body, err := io.ReadAll(r.Body)
		config.Handle(err, "Reading the body for new interface", false)

		if strings.Contains(string(body), "stop") {
			fmt.Println("GOT REQUEST TO STOP")
			go func() {
				stop <- struct{}{}
				fmt.Println("Sent signal to stop the goroutine")
				handle.Close()
				fmt.Println("Handle is closed")
			}()
			return
		} else if strings.Contains(string(body), "start") {
			fmt.Println("GOT REQUEST TO START")
			if listening {
				handle.Close()
				fmt.Println("Closed the handle before starting")
			}
			SSEClean()
			// for i := 1; i <= 10; i++ {
			// 	packetInfo.Protocol = "SSE_CLEAN"
			// 	packetInfo.Interface = "SSE_CLEAN"
			// 	fmt.Println("SENT SSE_CLEAN NUMBER: ", i)
			// 	if i == 10 {
			// 		packetInfo.SrcAddr = "done"
			// 	}
			// 	messageChan <- packetInfo
			// }
			// fmt.Println("FINISHED CLEANING SSE")

			//SSEClean()
			go listenPackets()
		} else {
			go func() {
				fmt.Println("Sending stop signal")
				switch listening {
				case true:
					handle.Close() //close handle just in case
					fmt.Println("Stopped successfully")
					embedded_db.UpdateDatabase(db, "iface", newiface)
					embedded_db.UpdateDatabase(db, "filter", newfilterstring)
					if newTimeMethod == "on" {
						embedded_db.UpdateDatabase(db, "time_method", "packet_timestamp")
					} else {
						embedded_db.UpdateDatabase(db, "time_method", "packet_proccessed_timestamp")
					}
					fmt.Println("Updated the embedded db")
					y = 1
					SSEClean()
					fmt.Println("FINISHED CLEANING SSE")
					if newfilterstring == "none" {
						newfilterstring = ""
					}
					time.Sleep(time.Second)
					readyChan <- "true"
				case false:
					fmt.Println("Goroutine hasn't started")
					embedded_db.UpdateDatabase(db, "iface", newiface)
					embedded_db.UpdateDatabase(db, "filter", newfilterstring)
					if newTimeMethod == "on" {
						embedded_db.UpdateDatabase(db, "time_method", "packet_timestamp")
					} else {
						embedded_db.UpdateDatabase(db, "time_method", "packet_proccessed_timestamp")
					}
					SSEClean()
					fmt.Println("Updated the embedded db")
					y = 1
					readyChan <- "true"
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
	packetNumber := r.URL.Query().Get("packetnumber")
	if packetNumber == "clear" {
		for _, value := range packetsInDB {
			err := embedded_db.DeleteDatabase(db, value)
			if err != nil {
				return
			}
		}
		fmt.Println("CLEARED all of packetsInDB:", packetsInDB)
		packetsInDB = []string{} //reset it
	} else if packetNumber == "list" {
		embedded_db.ViewDatabase(db)
	} else {
		packetInfo, err := embedded_db.SearchDatabase(db, packetNumber)
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(responseJSON)
		fmt.Println("Successfully found packet information and it is sent")
	}

}

func SSEClean() {
	for i := 1; i <= 10; i++ {
		packetInfo.Protocol = "SSE_CLEAN"
		packetInfo.Interface = "SSE_CLEAN"
		fmt.Println("SENT SSE_CLEAN NUMBER: ", i)
		if i == 10 {
			packetInfo.SrcAddr = "done"
		}
		messageChan <- packetInfo
	}
	fmt.Println("FINISHED CLEANING SSE")
}
