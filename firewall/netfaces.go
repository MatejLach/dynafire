package firewall

import "net"

type NetInterface struct {
	Name      string
	Addresses []string
}

// TODO: Add config option to bind to a specific network interface
func ListNetInterfaces() ([]NetInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	result := make([]NetInterface, 0)
	for _, netface := range interfaces {
		addrs, err := netface.Addrs()
		if err != nil {
			return nil, err
		}

		if len(addrs) == 0 {
			continue
		}

		addresses := make([]string, 0)
		for _, addr := range addrs {
			addresses = append(addresses, addr.String())
		}

		netf := NetInterface{
			Name:      netface.Name,
			Addresses: addresses,
		}

		result = append(result, netf)
	}

	return result, err
}
