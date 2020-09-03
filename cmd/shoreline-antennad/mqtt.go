package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/hashicorp/mdns"
	"github.com/miekg/dns"
)

func resolveMDNS(host string) ([]net.IP, error) {
	mCast := &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: 5353,
	}

	uConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		return nil, err
	}
	defer uConn.Close()

	msg := new(dns.Msg)
	name := dns.Fqdn(host)
	msg.SetQuestion(name, dns.TypeA)
	data, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	_, err = uConn.WriteToUDP(data, mCast)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 65536)
	n, err := uConn.Read(buf)
	if err != nil {
		return nil, err
	}

	err = msg.Unpack(buf[:n])
	if err != nil {
		return nil, err
	}

	var result []net.IP
	for _, r := range msg.Answer {
		a, ok := r.(*dns.A)
		if !ok {
			continue
		}
		if a.Hdr.Name != name {
			continue
		}

		result = append(result, a.A)
	}

	return result, nil
}
func resolveLookup(hostport string) ([]string, error) {
	host, port, _ := net.SplitHostPort(hostport)
	if port == "" {
		port = "1883"
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		addrs, err = resolveMDNS(host)
	}
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ip := range addrs {
		result = append(result, ip.String()+":"+port)
	}

	return result, nil
}
func mdnsLookup(service string) ([]string, error) {
	var results []string
	ch := make(chan *mdns.ServiceEntry)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for e := range ch {
			if e.AddrV4 == nil || e.AddrV4.IsUnspecified() {
				continue
			}
			log.Println("Found MQTT server:", e.Host, e.AddrV4)
			results = append(results, fmt.Sprintf("%s:%d", e.AddrV4.String(), e.Port))
		}
	}()

	err := mdns.Lookup(service, ch)
	close(ch)
	wg.Wait()
	return results, err
}

type goClient struct {
	mqtt.Client
}

func goify(tok mqtt.Token) error {
	tok.Wait()
	return tok.Error()
}
func (c *goClient) Connect() error { return goify(c.Client.Connect()) }

func (c *goClient) Publish(topic string, qos int, retain bool, payload []byte) error {
	topic = strings.TrimPrefix(topic, "/")
	log.Println("Publish:", topic, string(payload))
	return goify(c.Client.Publish(topic, byte(qos), retain, payload))
}
func (c *goClient) Subscribe(topic string, qos int, callback mqtt.MessageHandler) error {
	topic = strings.TrimPrefix(topic, "/")
	log.Println("Subscribe:", topic)
	c.Client.Subscribe(topic, byte(qos), callback)
	return nil
}
