package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/server/httputils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/parsers/filters"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/netlabel"
)

const (
	byID = iota
	byName
)

func (n *networkRouter) getNetworksList(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	filter := r.Form.Get("filters")
	netFilters, err := filters.FromParam(filter)
	if err != nil {
		return err
	}

	list := []*types.NetworkResource{}
	if names, ok := netFilters["name"]; ok {
		for _, name := range names {
			if nw, errRsp := findNetwork(n.netController, name, byName); errRsp == nil {
				list = append(list, buildNetworkResource(nw))
			}
		}
	} else if ids, ok := netFilters["id"]; ok {
		for _, id := range ids {
			// Return all the prefix-matching networks
			l := func(nw libnetwork.Network) bool {
				if strings.HasPrefix(nw.ID(), id) {
					list = append(list, buildNetworkResource(nw))
				}
				return false
			}
			n.netController.WalkNetworks(l)
		}
	} else {
		for _, nw := range n.netController.Networks() {
			list = append(list, buildNetworkResource(nw))
		}
	}
	return httputils.WriteJSON(w, http.StatusOK, list)
}

func (n *networkRouter) getNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	nw, err := findNetwork(n.netController, vars["id"], byID)
	if err != nil {
		return err
	}
	return httputils.WriteJSON(w, http.StatusOK, buildNetworkResource(nw))
}

func (n *networkRouter) postNetworkCreate(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var create types.NetworkCreate
	var warning string

	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
		return err
	}

	nw, err := findNetwork(n.netController, create.Name, byName)
	if _, ok := err.(libnetwork.ErrNoSuchNetwork); err != nil && !ok {
		return err
	}
	if nw != nil {
		if create.CheckDuplicate {
			return libnetwork.NetworkNameError(create.Name)
		}
		warning = fmt.Sprintf("Network with name %s (id : %s) already exists", nw.Name(), nw.ID())
	}

	processCreateDefaults(n.netController, &create)

	nw, err = n.netController.NewNetwork(create.Driver, create.Name, parseOptions(create.Options)...)
	if err != nil {
		return err
	}

	return httputils.WriteJSON(w, http.StatusCreated, &types.NetworkCreateResponse{
		ID:      nw.ID(),
		Warning: warning,
	})
}

func (n *networkRouter) postNetworkConnect(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var connect types.NetworkConnect
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	if err := json.NewDecoder(r.Body).Decode(&connect); err != nil {
		return err
	}

	nw, err := findNetwork(n.netController, vars["id"], byID)
	if err != nil {
		return err
	}

	container, err := n.daemon.Get(connect.Container)
	if err != nil {
		return fmt.Errorf("invalid container %s : %v", container, err)
	}
	return container.ConnectToNetwork(nw.Name())
}

func (n *networkRouter) postNetworkDisconnect(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	var connect types.NetworkConnect
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	if err := httputils.CheckForJSON(r); err != nil {
		return err
	}

	if err := json.NewDecoder(r.Body).Decode(&connect); err != nil {
		return err
	}

	nw, err := findNetwork(n.netController, vars["id"], byID)
	if err != nil {
		return err
	}

	container, err := n.daemon.Get(connect.Container)
	if err != nil {
		return fmt.Errorf("invalid container %s : %v", container, err)
	}
	return container.DisconnectFromNetwork(nw)
}

func (n *networkRouter) deleteNetwork(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) error {
	if err := httputils.ParseForm(r); err != nil {
		return err
	}

	nw, err := findNetwork(n.netController, vars["id"], byID)
	if err != nil {
		return err
	}

	return nw.Delete()
}

func findNetwork(c libnetwork.NetworkController, s string, by int) (libnetwork.Network, error) {
	switch by {
	case byID:
		return c.NetworkByID(s)
	case byName:
		if s == "" {
			s = c.Config().Daemon.DefaultNetwork
		}
		return c.NetworkByName(s)
	}
	return nil, errors.New("unexpected selector for network search")
}

func buildNetworkResource(nw libnetwork.Network) *types.NetworkResource {
	r := &types.NetworkResource{}
	if nw != nil {
		r.Name = nw.Name()
		r.ID = nw.ID()
		r.Driver = nw.Type()
		r.Containers = make(map[string]types.EndpointResource)
		epl := nw.Endpoints()
		for _, e := range epl {
			sb := e.Info().Sandbox()
			if sb == nil {
				continue
			}

			er := types.EndpointResource{}
			er.EndpointID = e.ID()
			if iface := e.Info().Iface(); iface != nil {
				if mac := iface.MacAddress(); mac != nil {
					er.MacAddress = mac.String()
				}
				if ip := iface.Address(); len(ip.IP) > 0 {
					er.IPv4Address = (&ip).String()
				}

				if ipv6 := iface.AddressIPv6(); len(ipv6.IP) > 0 {
					er.IPv6Address = (&ipv6).String()
				}
			}
			r.Containers[sb.ContainerID()] = er
		}
	}
	return r
}

func processCreateDefaults(c libnetwork.NetworkController, n *types.NetworkCreate) {
	if n.Driver == "" {
		n.Driver = c.Config().Daemon.DefaultDriver
	}

	if n.Options == nil {
		n.Options = make(map[string]interface{})
	}
	genericData, ok := n.Options[netlabel.GenericData]
	if !ok {
		genericData = make(map[string]interface{})
	}
	n.Options[netlabel.GenericData] = genericData
}

func parseOptions(options map[string]interface{}) []libnetwork.NetworkOption {
	var setFctList []libnetwork.NetworkOption

	if options != nil {
		setFctList = append(setFctList, libnetwork.NetworkOptionGeneric(options))
	}

	return setFctList
}
