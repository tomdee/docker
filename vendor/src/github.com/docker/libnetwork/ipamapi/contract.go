// Package ipamapi specifies the contract the IPAM service (built-in or remote) needs to satisfy.
package ipamapi

import (
	"errors"
	"net"
)

/********************
 * IPAM plugin types
 ********************/

const (
	// DefaultIPAM is the name of the built-in default ipam driver
	DefaultIPAM = "default"
	// PluginEndpointType represents the Endpoint Type used by Plugin system
	PluginEndpointType = "IPAM"
)

// Callback provides a Callback interface for registering an IPAM instance into LibNetwork
type Callback interface {
	// RegisterDriver provides a way for Remote drivers to dynamically register new NetworkType and associate with a ipam instance
	RegisterIpamDriver(name string, config Config, allocator Allocator) error
}

/**************
 * IPAM Errors
 **************/

// Weel-known errors returned by IPAM
var (
	ErrInvalidIpamService       = errors.New("Invalid IPAM Service")
	ErrInvalidIpamConfigService = errors.New("Invalid IPAM Config Service")
	ErrIpamNotAvailable         = errors.New("IPAM Service not available")
	ErrIpamInternalError        = errors.New("IPAM Internal Error")
	ErrInvalidAddressSpace      = errors.New("Invalid Address Space")
	ErrInvalidPool              = errors.New("Invalid Address Pool")
	ErrInvalidSubPool           = errors.New("Invalid Address SubPool")
	ErrInvalidRequest           = errors.New("Invalid Request")
	ErrPoolNotFound             = errors.New("Address Pool not found")
	ErrOverlapPool              = errors.New("Address pool overlaps with existing pool on this address space")
	ErrNoAvailablePool          = errors.New("No available pool")
	ErrNoAvailableIPs           = errors.New("No available addresses on this pool")
	ErrIPAlreadyAllocated       = errors.New("Address already in use")
	ErrIPOutOfRange             = errors.New("Requested address is out of range")
	ErrPoolOverlap              = errors.New("Pool overlaps with other one on this address space")
	ErrBadPool                  = errors.New("Address space does not contain specified address pool")
)

/*******************************
 * IPAM Configuration Interface
 *******************************/

// Config represents the interface the IPAM service plugins must implement
// in order to allow injection/modification of IPAM database.
// Common key is an address space. An address space is a set of non-overlapping address pools.
type Config interface {
	// GetDefaultAddressSpaces returns the local and global default address spaces
	GetDefaultAddressSpaces() (string, string, error)
	// RequestPool returns an address pool along with its unique id.
	RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error)
	// ReleasePool releases the address pool identified by the passed id
	ReleasePool(poolID string) error
}

/*************************
 * IPAM Service Interface
 *************************/

// Allocator defines the interface the IPAM service plugins must implement
// Common key is a unique address space identifier
type Allocator interface {
	// Request address from the specified pool ID. Input options or preferred IP can be passed.
	RequestAddress(string, net.IP, map[string]string) (*net.IPNet, map[string]string, error)
	// Release the address from the specified pool ID
	ReleaseAddress(string, net.IP) error
}
