package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Radictionary/website/models"
	"github.com/Radictionary/website/packet"
	"github.com/Radictionary/website/pkg/config"
	"github.com/Radictionary/website/pkg/render"
	"github.com/Radictionary/website/pkg/template_models"
	"github.com/Radictionary/website/redis"
	"github.com/Radictionary/website/sse"

	"github.com/google/gopacket/pcap"
)

var (
	stop              = make(chan struct{})
	packetInfo        models.PacketStruct
	settingsRetrieval models.SettingsRetrieval
	listening         bool = false
	packetNumber           = new(int)
	MessageChan            = make(chan models.PacketStruct)
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

// Home is the handler for the home page
func (m *Repository) Home(w http.ResponseWriter, r *http.Request) {
	err := redis.InitRedisConnection()
	config.Handle(err, "Redis connection error", false)
	*packetNumber = 1
	go sse.StartSSE()
	render.RenderTemplate(w, "home.html", &template_models.TemplateData{})
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
	sse.RegisterChan <- w
	defer func() {
		sse.UnregisterChan <- w
	}()
	for packet := range MessageChan {
		sse.SendToAllClients("new-packet", packet)
		flusher.Flush()
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
		body, err := io.ReadAll(r.Body)
		config.Handle(err, "Reading the body", false)

		var data map[string]interface{}
		_ = json.Unmarshal(body, &data)
		// Check if the "fullPath" key exists in the parsed data, and only then will full path be set
		if fullPath, ok := data["fullPath"].(string); ok {
			redis.StoreData("savePath", fullPath)
		} else if save, ok := data["save"].(string); ok {
			packet.SavePackets(save)
		}

		if strings.Contains(string(body), "stop") {
			if listening {
				stop <- struct{}{}
			}
			listening = false
			return
		} else if strings.Contains(string(body), "start") {
			if listening {
				stop <- struct{}{}
			}
			go packet.ListenPackets(packetInfo, packetNumber, stop, MessageChan)
			listening = true
		} else if strings.Contains(string(body), "save") {
			//packet.SavePackets()
		} else {
			go func() {
				if listening {
					stop <- struct{}{}
				}
				if newiface != "" {
					redis.StoreData("interface", newiface)
				}
				if newfilterstring != "" && newfilterstring != "none" {
					redis.StoreData("filter", newfilterstring)
				} else if newfilterstring == "none" {
					redis.StoreData("filter", "")
				}
				if listening {
					go packet.ListenPackets(packetInfo, packetNumber, stop, MessageChan)
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
	neededpacketNumber := r.URL.Query().Get("packetnumber")
	if neededpacketNumber == "clear" {
		redis.ClearPackets("packet")
		redis.ClearPackets("packetsFromFile")
		*packetNumber = 1
		w.WriteHeader(http.StatusOK)
		return
	}
	result, err := redis.RetrieveStruct("packet:" + neededpacketNumber)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println("Error retrieving packet from redis in json format:", err)
		return
	}
	packetStruct, err := json.Marshal(result)
	config.Handle(err, "Json Marshaling PacketStruct", false)
	w.WriteHeader(http.StatusOK)
	w.Write(packetStruct)
}

func (m *Repository) SettingsSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	iface, err := redis.RetrieveData("interface")
	config.Handle(err, "searching the database for iface", false)
	settingsRetrieval.Interface = iface

	filter, err := redis.RetrieveData("filter")
	config.Handle(err, "searching the database for iface", false)

	settingsRetrieval.Filter = filter

	fileSave, err := redis.RetrieveData("savePath")
	if err != nil && !strings.Contains(err.Error(), "does not exist") {
		fmt.Println("ERROR searching for savePath:", err)
	}
	settingsRetrieval.PacketSaveDir = fileSave

	jsonPayload, err := json.Marshal(settingsRetrieval)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(jsonPayload)
}

func (m *Repository) Upload(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to retrieve uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	tempFile, err := os.CreateTemp("", "uploaded.pcap")
	if err != nil {
		http.Error(w, "Failed to create temporary file", http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	// Copy the uploaded file to the temporary file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// Open the temporary file as a pcap packet source
	handle, err := pcap.OpenOffline(tempFile.Name())
	if err != nil {
		http.Error(w, "Failed to open pcap file", http.StatusInternalServerError)
		return
	}
	defer handle.Close()
	packet.ListenPacketsFromFile(handle, packetInfo, MessageChan)
	// Delete the temporary file after processing
	os.Remove(tempFile.Name())

	w.WriteHeader(http.StatusOK)
}

func (m *Repository) Retrieve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	get := r.URL.Query().Get("get")
	if get == "recover" {
		packets, err := redis.RecoverPackets("packet")
		if err != nil {
			fmt.Println("Error recovering packets from redis function")
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if len(packets) == 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("[]"))
			return
		}
		*packetNumber = len(packets) + 1
		packetsJSON, err := json.Marshal(packets)
		if err != nil {
			fmt.Println("Error marshalling recovered packets:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(packetsJSON)
	} else if get == "filecontents" {
		packets, _ := redis.RecoverPackets("packetsFromFile")
		packetsJSON, _ := json.Marshal(packets)
		w.Write([]byte(packetsJSON))
	}
}
