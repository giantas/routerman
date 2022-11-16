package main

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
	db     *storage.Store
	ctx    Context
	router *RouterApi
}

func NewEnv(in io.Reader, db *sql.DB, router *RouterApi) *Env {
	store := storage.NewStore(db)

	return &Env{
		in:     in,
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

func (slot BwSlot) GetMaxIP(numAddresses int) (string, error) {
	var maxIP string
	start, err := tplinkapi.Ip2Int(slot.MinAddress)
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

func (api RouterApi) GetAvailableBandwidthSlots() ([]BwSlot, error) {
	var slots []BwSlot
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
	startIp, err := tplinkapi.Ip2Int(lanConfig.MinAddress)
	if err != nil {
		return slots, err
	}
	endIp, err := tplinkapi.Ip2Int(lanConfig.MaxAddress)
	if err != nil {
		return slots, err
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
			slot = BwSlot{
				LanConfig: tplinkapi.LanConfig{
					MinAddress: start,
					MaxAddress: start,
					SubnetMask: lanConfig.SubnetMask,
				},
			}
		} else {
			prevItem := ipList[i-1]
			if prevItem == item-1 {
				slot.MaxAddress = itemString
			} else {
				slots = append(slots, slot)
				slot = BwSlot{
					LanConfig: tplinkapi.LanConfig{
						MinAddress: itemString,
						MaxAddress: itemString,
						SubnetMask: lanConfig.SubnetMask,
					},
				}
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
	for k, _ := range ipRange {
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
