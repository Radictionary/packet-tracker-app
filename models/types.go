package models

type PacketStruct struct {
	Interface    string `json:"interface"`
	Protocol     string `json:"protocol"`
	SrcAddr      string `json:"srcAddr"`
	DstnAddr     string `json:"dstnAddr"`
	Length       int    `json:"length"`
	PacketNumber int    `json:"packetNumber"`
	PacketDump   string `json:"packetDump"`
	PacketData []byte
	Time         string `json:"time"`
	Err          string `json:"err"`
	Saved        bool   `json:"saved"`
}

type SettingsRetrieval struct {
	Interface     string `json:"interface"`
	Filter        string `json:"filter"`
	PacketSaveDir string `json:"packetSaveDir"`
}
