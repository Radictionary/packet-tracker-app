package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

)

var (
	Clients        = make(map[http.ResponseWriter]struct{})
	ClientsMutex   sync.Mutex
	RegisterChan   = make(chan http.ResponseWriter)
	UnregisterChan = make(chan http.ResponseWriter)
)


func SendToClient(client http.ResponseWriter, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Printf("Error marshaling event data to JSON: %v\n", err)
		UnregisterChan <- client
		return
	}

	_, err = fmt.Fprintf(client, "event: %s\ndata: %s\n\n", event, jsonData)
	if err != nil {
		fmt.Printf("Error sending SSE to client: %v\n", err)
		UnregisterChan <- client
	}

	flusher, ok := client.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}
func SendToAllClients(event string, data interface{}) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	for client := range Clients {
		SendToClient(client, event, data)
	}
}
func RegisterClient(w http.ResponseWriter) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	Clients[w] = struct{}{}
}
func UnregisterClient(w http.ResponseWriter) {
	ClientsMutex.Lock()
	defer ClientsMutex.Unlock()

	delete(Clients, w)
}
func StartSSE() {
	for {
		select {
		case client := <-RegisterChan:
			RegisterClient(client)
		case client := <-UnregisterChan:
			UnregisterClient(client)
		}
	}
}