package cli

import (
	"database/sql"
	"fmt"
	"io"
	"sort"

	"github.com/omushpapa/routerman/storage"
	"github.com/omushpapa/tplinkapi"
)

type Env struct {
	in     io.Reader
	out    io.Writer
	db     *storage.Store
	ctx    Context
	router *RouterApi
}

func NewEnv(in io.Reader, out io.Writer, db *sql.DB, router *RouterApi) *Env {
	store := storage.NewStore(db)

	return &Env{
		in:     in,
		out:    out,
		db:     store,
		ctx:    make(Context),
		router: router,
	}
}

type BwSlot struct {
	tplinkapi.LanConfig
}

func (slot BwSlot) GetCapacity() (int, error) {
	start, err := tplinkapi.Ip2Int(slot.MinAddress)
	if err != nil {
		return 0, err
	}
	stop, err := tplinkapi.Ip2Int(slot.MaxAddress)
	if err != nil {
		return 0, err
	}
	capacity := int(stop-start) + 1
	return capacity, nil
}

func (slot BwSlot) GetMaxIP(startIP string, numAddresses int) (string, error) {
	var maxIP string
	start, err := tplinkapi.Ip2Int(startIP)
	if err != nil {
		return maxIP, err
	}
	end := start + uint32(numAddresses)
	maxIP = tplinkapi.Int2ip(end).String()
	return maxIP, err
}

type RouterApi struct {
	service tplinkapi.RouterService
}

func NewRouterApi(username, password, address string) *RouterApi {
	service := tplinkapi.RouterService{
		Username: username,
		Password: password,
		Address:  address,
	}
	return &RouterApi{service: service}
}

func (api RouterApi) GetAvailableBandwidthSlots(useDhcpBounds bool) ([]BwSlot, error) {
	var (
		slots   []BwSlot
		startIp uint32
		endIp   uint32
		err     error
	)
	info, err := api.service.GetRouterInfo()
	if err != nil {
		return slots, err
	}
	routerIp, err := tplinkapi.Ip2Int(info.IP)
	if err != nil {
		return slots, err
	}
	lanConfig, err := api.service.GetLanConfig()
	if err != nil {
		return slots, err
	}
	details, err := api.service.GetBandwidthControlDetails()
	if err != nil {
		return slots, err
	}
	if useDhcpBounds {
		startIp, err = tplinkapi.Ip2Int(lanConfig.MinAddress)
		if err != nil {
			return slots, err
		}
		endIp, err = tplinkapi.Ip2Int(lanConfig.MaxAddress)
		if err != nil {
			return slots, err
		}
	} else {
		startIp = routerIp + 1
		prefix := lanConfig.GetPrefix()
		if prefix == 0 {
			return slots, fmt.Errorf("invalid subnet prefix '%d'", prefix)
		}
		networkSize := 1 << (32 - prefix)
		endIp = startIp + (uint32(networkSize) - 3)
	}
	allIps := make(map[uint32]bool)
	for i := startIp; i < endIp+1; i++ {
		allIps[i] = true
	}

	_, exists := allIps[routerIp]
	if exists {
		delete(allIps, routerIp)
	}

	for _, entry := range details.Entries {
		if !entry.Enabled {
			continue
		}

		start, err := tplinkapi.Ip2Int(entry.StartIp)
		if err != nil {
			return slots, err
		}
		end, err := tplinkapi.Ip2Int(entry.EndIp)
		if err != nil {
			return slots, err
		}
		for i := start; i < end+1; i++ {
			_, exists := allIps[i]
			if exists {
				delete(allIps, i)
			}
		}
	}

	ipList := make([]uint32, 0)
	for k := range allIps {
		ipList = append(ipList, k)
	}
	sort.Slice(ipList, func(i, j int) bool {
		return ipList[i] < ipList[j]
	})

	var slot BwSlot
	for i, item := range ipList {
		itemString := tplinkapi.Int2ip(item).String()

		if i == 0 {
			start := itemString
			cfg, err := tplinkapi.NewLanConfig(start, start, lanConfig.SubnetMask)
			if err != nil {
				return slots, err
			}

			slot = BwSlot{LanConfig: cfg}
		} else {
			prevItem := ipList[i-1]
			if prevItem == item-1 {
				slot.MaxAddress = itemString
			} else {
				slots = append(slots, slot)
				cfg, err := tplinkapi.NewLanConfig(itemString, itemString, lanConfig.SubnetMask)
				if err != nil {
					return slots, err
				}

				slot = BwSlot{LanConfig: cfg}
			}

			if i == len(ipList)-1 {
				slots = append(slots, slot)
			}
		}
	}
	return slots, nil
}

func (api RouterApi) GetBwControlEntriesByList(ids []int) ([]tplinkapi.BandwidthControlEntry, error) {
	entries := make([]tplinkapi.BandwidthControlEntry, 0)
	details, err := api.service.GetBandwidthControlDetails()
	if err != nil {
		return entries, err
	}
	remoteEntries := make(map[int]tplinkapi.BandwidthControlEntry)
	for _, entry := range details.Entries {
		remoteEntries[entry.Id] = entry
	}
	for _, id := range ids {
		entry, exists := remoteEntries[id]
		if !exists {
			return entries, fmt.Errorf("entry with id '%d' not found", id)
		}
		entries = append(entries, entry)
	}
	return entries, err
}

func (api RouterApi) GetUnusedIPAddress(slotId int) (string, error) {
	entry, err := api.service.GetBandwidthControlEntry(slotId)
	if err != nil {
		return "", err
	}
	reservations, err := api.service.GetAddressReservations()
	if err != nil {
		return "", err
	}
	startIpInt, err := tplinkapi.Ip2Int(entry.StartIp)
	if err != nil {
		return "", err
	}
	endIpInt, err := tplinkapi.Ip2Int(entry.EndIp)
	if err != nil {
		return "", err
	}
	ipRange := make(map[uint32]bool, 0)
	for i := startIpInt; i <= endIpInt; i++ {
		ipRange[i] = true
	}
	for _, resv := range reservations {
		ipInt, err := tplinkapi.Ip2Int(resv.IP)
		if err != nil {
			return "", err
		}
		_, exists := ipRange[ipInt]
		if exists {
			delete(ipRange, ipInt)
		}
	}
	validIps := make([]uint32, 0)
	for k := range ipRange {
		validIps = append(validIps, k)
	}
	sort.Slice(validIps, func(i, j int) bool {
		return validIps[i] < validIps[j]
	})
	if len(validIps) == 0 {
		return "", fmt.Errorf("no ip addresses available")
	}
	return tplinkapi.Int2ip(validIps[0]).String(), err
}

func (api RouterApi) BlockDevice(macAddress string) error {
	if !IsValidMacAddress(macAddress) {
		return fmt.Errorf("invalid mac address")
	}

	cfg := tplinkapi.InternetAccessControl{
		Enabled:     true,
		DefaultDeny: false,
	}
	if err := api.service.ToggleInternetAccessControl(cfg); err != nil {
		return fmt.Errorf("error while toggling internet access '%v' ", err)
	}

	hosts, err := api.service.GetAccessControlHosts()
	if err != nil {
		return err
	}

	var host tplinkapi.MacAddressAccessControlHost
	deviceHosts := hosts[tplinkapi.MacAddressHostType]
	for _, deviceHost := range deviceHosts {
		if d, ok := deviceHost.(tplinkapi.MacAddressAccessControlHost); ok {
			if d.Mac == macAddress {
				host = d
				break
			}
		}
	}

	if host.Id == 0 {
		host, err = tplinkapi.NewMacAddressAccessControlHost(macAddress)
		if err != nil {
			return err
		}

		if _, err = api.service.AddAccessControlHost(host); err != nil {
			return fmt.Errorf("error while adding access control host '%v' ", err)
		}
	}

	if _, err = api.service.AddAccessControlRule(host); err != nil {
		return fmt.Errorf("error while adding access control rule '%v' ", err)
	}
	fmt.Printf("device '%s' blocked\n", macAddress)
	return nil
}

func (api RouterApi) UnblockDevice(macAddress string) error {
	if !IsValidMacAddress(macAddress) {
		return fmt.Errorf("invalid mac address")
	}

	hosts, err := api.service.GetAccessControlHosts()
	if err != nil {
		return err
	}

	var host tplinkapi.MacAddressAccessControlHost
	validHosts := hosts[tplinkapi.MacAddressHostType]
	for _, v := range validHosts {
		if h, ok := v.(tplinkapi.MacAddressAccessControlHost); ok {
			if h.Mac == macAddress {
				host = h
				break
			}
		}
	}

	if host.Id == 0 {
		return fmt.Errorf("host with mac '%s' not found", macAddress)
	}

	rules, err := api.service.GetAccessControlRules()
	if err != nil {
		return err
	}

	hostRef := host.GetRef()

	var rule tplinkapi.AccessControlRule
	for _, r := range rules {
		if r.InternalHostRef == hostRef {
			rule = r
			break
		}
	}

	if rule.Id == 0 {
		return fmt.Errorf("rule for host with ref '%s' not found", hostRef)
	}

	err = api.service.DeleteAccessControlRule(rule.Id)
	return err
}

func (api RouterApi) GetBlockedDevices() ([]string, error) {
	devices := make([]string, 0)

	hosts, err := api.service.GetAccessControlHosts()
	if err != nil {
		return devices, err
	}

	if len(hosts) == 0 {
		return devices, nil
	}

	rules, err := api.service.GetAccessControlRules()
	if err != nil {
		return devices, err
	}

	if len(rules) == 0 {
		return devices, nil
	}

	refs := make(map[string]bool, 0)
	for _, rule := range rules {
		refs[rule.InternalHostRef] = true
	}

	deviceHosts := hosts[tplinkapi.MacAddressHostType]
	for _, host := range deviceHosts {
		if h, ok := host.(tplinkapi.MacAddressAccessControlHost); ok {
			ref := h.GetRef()

			if _, ok := refs[ref]; ok {
				devices = append(devices, h.Mac)
			}
		}
	}
	return devices, nil
}
