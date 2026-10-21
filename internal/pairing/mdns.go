package pairing

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/mdns"
)

// ServiceType is the mDNS service type Runyx advertises/discovers under.
const ServiceType = "_runyx._tcp"

// Announcer advertises this machine's Runyx daemon on the local network
// via mDNS, so the Android app can find it without the user typing an
// IP address.
type Announcer struct {
	server *mdns.Server
}

// Announce starts advertising host:port under ServiceType. instanceName
// should be unique per machine (e.g. the hostname).
func Announce(instanceName string, port int) (*Announcer, error) {
	info := []string{"Runyx local LLM daemon"}
	service, err := mdns.NewMDNSService(instanceName, ServiceType, "", "", port, nil, info)
	if err != nil {
		return nil, fmt.Errorf("create mdns service: %w", err)
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("start mdns server: %w", err)
	}
	return &Announcer{server: server}, nil
}

// Stop halts the announcement.
func (a *Announcer) Stop() error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown()
}

// Discovered is one Runyx instance found on the local network.
type Discovered struct {
	Name string
	Host string
	Port int
}

// Discover browses the local network for Runyx instances for the given
// duration, returning whatever was found. Used by companion
// clients (the Android app) to locate a PC without pairing yet.
func Discover(ctx context.Context, timeout time.Duration) ([]Discovered, error) {
	entriesCh := make(chan *mdns.ServiceEntry, 16)
	var out []Discovered
	done := make(chan struct{})

	go func() {
		defer close(done)
		for entry := range entriesCh {
			host := entry.AddrV4.String()
			if host == "<nil>" || host == "" {
				host = entry.Host
			}
			out = append(out, Discovered{Name: entry.Name, Host: host, Port: entry.Port})
		}
	}()

	params := mdns.DefaultParams(ServiceType)
	params.Entries = entriesCh
	params.Timeout = timeout
	if err := mdns.Query(params); err != nil {
		close(entriesCh)
		return nil, fmt.Errorf("mdns query: %w", err)
	}
	close(entriesCh)
	<-done
	return out, nil
}
