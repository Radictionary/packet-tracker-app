package packet

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Radictionary/website/models"
	"github.com/Radictionary/website/pkg/config"
	"github.com/Radictionary/website/redis"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/pcapgo"
)

// DetectProtocol detects the protocol and returns what it is
func DetectProtocol(packet gopacket.Packet) (string, string, string) {
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
			udp, _ := transport.(*layers.UDP)
			sourceAddress = packet.NetworkLayer().NetworkFlow().Src().String()
			destAddress = packet.NetworkLayer().NetworkFlow().Dst().String()

			// Check for DNS packets
			if udp.DstPort == 53 || udp.SrcPort == 53 {
				protocol = "DNS"
			}
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

// ListenPackets function listens for packets in the background and sends packets to the frontend via SSE
func ListenPackets(packetStruct models.PacketStruct, packetNumber *int, stop chan struct{}, MessageChan chan models.PacketStruct) {
	err := redis.InitRedisConnection()
	config.Handle(err, "Initializing redis connection", false)
	iface, err := redis.RetrieveData("interface")
	config.Handle(err, "searching the database for iface", false)
	filter, err := redis.RetrieveData("filter")
	config.Handle(err, "searching the database for filter", false)
	file_save, err := redis.RetrieveData("savePath")
	if err != nil && !strings.ContainsAny(err.Error(), "does not exist") {
		fmt.Println("ERROR searching for savePath:", err)
	}

	var (
		snaplen      = int32(1600)
		promisc      = false
		timeout      = pcap.BlockForever
		devFound     = false
		interfaceErr bool
		filterErr    bool
	)
	if iface == "" {
		redis.StoreData("interface", "en0")
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
		config.Handle(err, "Device selected does not exist", false)
	}
	handle, err := pcap.OpenLive(iface, snaplen, promisc, timeout)
	config.Handle(err, "Finding all devices", true)

	if err := handle.SetBPFFilter(filter); err != nil {
		fmt.Println("Couldn't filter with current settings. Reseting the filter to be nothing. The filter was: ", filter)
		redis.StoreData("filter", "")

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

	for singlePacket := range source.Packets() {
		select {
		case <-stop:
			return
		default:
			var protocol string
			protocol, packetStruct.SrcAddr, packetStruct.DstnAddr = DetectProtocol(singlePacket)
			if filterErr {
				packetStruct.Err = "Filter was invalid. Reset the filter."
			} else if interfaceErr {
				packetStruct.Err = "Interface was invalid. Reset the interface to en0."
			}
			packetStruct.Protocol = protocol
			packetStruct.PacketNumber = *packetNumber
			packetStruct.Time = singlePacket.Metadata().Timestamp.Format("15:04:05")
			packetStruct.Interface = iface
			packetStruct.Length = singlePacket.Metadata().Length
			packetStruct.PacketDump = singlePacket.Dump()
			packetStruct.PacketData = singlePacket.Data()

			if file_save != "" {
				err := pcapWriter.WritePacket(singlePacket.Metadata().CaptureInfo, singlePacket.Data())
				if err != nil {
					config.Handle(err, "Writing packet to pcap file", false)
				}
				packetStruct.Saved = true
			} else {
				packetStruct.Saved = false
			}
			MessageChan <- packetStruct
			redis.HashStruct(packetStruct, "packet")
			*packetNumber++
		}
	}
}

// ListenPacketsFromFile function handles packets from a pcap file
func ListenPacketsFromFile(handle *pcap.Handle, packetStruct models.PacketStruct, MessageChan chan models.PacketStruct) {
	var openedPacketsfromFile int = 1
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for Singlepacket := range packetSource.Packets() {
		var protocol string
		protocol, packetStruct.SrcAddr, packetStruct.DstnAddr = DetectProtocol(Singlepacket)
		packetStruct.Protocol = protocol
		packetStruct.PacketNumber = openedPacketsfromFile
		packetStruct.Time = Singlepacket.Metadata().Timestamp.Format("15:04:05")
		packetStruct.Interface = "N/A"
		packetStruct.Length = Singlepacket.Metadata().Length
		packetStruct.PacketDump = Singlepacket.Dump()
		packetStruct.Saved = true
		redis.HashStruct(packetStruct, "packetsFromFile")
		openedPacketsfromFile++
	}
}

func SavePackets(file_save string) {
	redis.InitRedisConnection()
	packets, err := redis.RecoverPackets("packet")
	if err != nil {
		fmt.Println("Error recovering packets from redis function:", err)
		return
	} else if len(packets) == 0 {
		return
	}

	pcapFile, err := os.Create(file_save + ".pcap")
	if err != nil {
		config.Handle(err, "Creating pcap file", true)
	}
	defer pcapFile.Close()

	pcapWriter := pcapgo.NewWriter(pcapFile)
	config.Handle(err, "Creating pcap writer", false)

	snaplen := 65535
	linkType := layers.LinkTypeEthernet

	// Write pcap file headers
	if err := pcapWriter.WriteFileHeader(uint32(snaplen), linkType); err != nil {
		fmt.Println("Error writing file header:", err)
	}

	for _, packetData := range packets {
		time, _ := time.Parse(time.RFC1123, packetData.Time)
		captureInfo := gopacket.CaptureInfo{
			Timestamp:      time, // Set the packet timestamp if available
			CaptureLength:  len(packetData.PacketData),
			Length:         len(packetData.PacketData),
		}
		if err := pcapWriter.WritePacket(captureInfo, packetData.PacketData); err != nil {
			fmt.Println("Error writing packet to file:", err)
		}
	}
	redis.MarkAsSaved()
}

