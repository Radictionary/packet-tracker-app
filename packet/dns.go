package packet

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/miekg/dns"
)

const dnsResolver = "1.1.1.1:53"

func DnsInformation(domain string) (*dns.Msg, error) {
	var result *dns.Msg
	var err error
	typeA := new(dns.Msg)
	typeA.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	typeCert := new(dns.Msg)
	typeCert.SetQuestion(dns.Fqdn(domain), dns.TypeCERT)

	c := new(dns.Client)
	result, _, err = c.Exchange(typeA, dnsResolver)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ProcessDnsPacket(packetData []byte) *layers.DNS {
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)

	// Retrieve the DNS layer from the packet
	dnsLayer := packet.Layer(layers.LayerTypeDNS)

	dnsPacket, _ := dnsLayer.(*layers.DNS)
	return dnsPacket
}
