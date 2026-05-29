package domain

import "net"

// Device — entity, представляющая устройство в LAN. Идентифицируется по MAC
// (стабильный идентификатор в отличие от IP, который может меняться по DHCP).
// Hostname и IP опциональны — могут отсутствовать в DHCP leases.
type Device struct {
	MAC      MAC
	Hostname string
	IP       net.IP
}
