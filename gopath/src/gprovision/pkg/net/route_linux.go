// Copyright (C) 2015-2020 the Gprovision Authors. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// SPDX-License-Identifier: BSD-3-Clause
//

package net

import (
	"fmt"
	"gprovision/pkg/common"
	"gprovision/pkg/hw/nic"
	"gprovision/pkg/log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const (
	defaultVia = "default via"
	maxTries   = 32 //used in adjustMetric(), called by AdjustDupDefaultRoutes()
)

// Route represents an IP route.
type Route struct {
	Iface  string        `json:",omitempty"`
	Gw     net.IP        `json:",omitempty"`
	Proto  int           `json:",omitempty"`
	Metric uint64        `json:",omitempty"`
	Src    net.IP        `json:",omitempty"`
	Dest   IPNet         `json:",omitempty"`
	Scope  netlink.Scope `json:",omitempty"`
}

func (l Route) Equal(r Route) bool {
	switch {
	case l.Iface != r.Iface:
		return false
	case l.Proto != r.Proto:
		return false
	case l.Scope != r.Scope:
		return false
	case l.Metric != r.Metric:
		return false
	case !l.Gw.Equal(r.Gw):
		return false
	case !l.Src.Equal(r.Src):
		return false
	case !l.Dest.Equal(r.Dest):
		return false
	}
	return true
}

//used by AdjustDupDefaultRoutes to avoid repeated WAN lookup.
var wan string

// Remove duplicate routes if MetricForDuplicates is 0, else give them different
// priority by changing the metric. (high metric -> low prio)
func AdjustDupDefaultRoutes(plat common.PlatInfoer, MetricForDuplicates uint64) {
	if wan == "" {
		w := WanDevice(plat)
		if w != nil {
			wan = w.Name()
		}
	}
	if wan == "" {
		log.Logf("failed to find WAN port, will not remove any routes")
		return
	}
	routes := GetDefaultRoutes()
	var keep Route
	for _, r := range routes {
		if r.Iface == wan {
			keep = r
			break
		}
	}
	for _, r := range routes {
		if r.Proto == unix.RTPROT_STATIC {
			//ignore any routes that look duplicate but are static
			log.Logf("Ignoring duplicate route '%s'", r.String())
			continue
		}
		if r.Iface != keep.Iface {
			if MetricForDuplicates == 0 {
				log.Logf("Removing duplicate route '%s'", r.String())
				r.Remove()
			} else if r.Metric != MetricForDuplicates {
				log.Logf("Adjusting metric for duplicate route '%s' to %d", r.String(), MetricForDuplicates)
				r.adjustMetric(MetricForDuplicates, 0)
			}
		}
	}
}

// Returns all 'default via ...' routes.
func GetDefaultRoutes() []Route { return GetRoutes(true) }

// Returns routes.
func GetRoutes(onlyDefault bool) []Route {
	list := exec.Command("ip", "route", "list")
	out, success := log.Cmd(list)
	if !success {
		return nil
	}
	return parseRoutes(out, onlyDefault)
}

func parseRoutes(output string, onlyDefault bool) []Route {
	var routes []Route
	for _, line := range strings.Split(output, "\n") {
		success, rt := parseRoute(strings.TrimSpace(line), onlyDefault)
		if success {
			routes = append(routes, rt)
		}
	}
	return routes
}

func parseRoute(line string, onlyDefault bool) (bool, Route) {
	if line == "" || line == "cache" {
		return false, Route{}
	}
	if onlyDefault && !strings.HasPrefix(line, defaultVia) {
		return false, Route{}
	}
	/*
		192.168.133.186 dev tun0 proto kernel scope link src 192.168.133.185
		8.8.8.8 via 4.3.2.1 dev enp4s0 src 10.254.64.174 uid 1000
		default via 4.3.2.1 dev eth0 proto static
		default via 4.3.2.1 dev enp2s0 proto dhcp src 4.3.2.3 metric 1024
		default via 4.3.2.1 dev eno1 proto dhcp src 4.3.2.2 metric 1024
		   0     1     2     3    4     5    6   7     8      9     10
		               gw        dev        dyn?    src addr       metric
		once you remove the first element, all seem to be in "key value" format
	*/
	elements := strings.Split(line, " ")
	if len(elements) < 7 {
		log.Logf("can't parse route, skipping - too few fields from %s", line)
		return false, Route{}
	}
	rt := Route{}
	if elements[0] != "default" {
		dest, err := IPNetFromCIDR(elements[0])
		if err != nil {
			log.Logf("failed to parse %s: %s", elements[0], err)
		} else {
			rt.Dest = dest
		}
	}
	/* Find src ip and metric, if given
	   The following assumes the output can take various forms other than those
	   above, so if we don't recognize something log it and continue. If no src
	   IP is provided, r.src will be nil; if no metric, r.metric will be 0; and
	   so on.
	*/
	elements = elements[1:]
	for len(elements) > 0 {
		success := false
		if len(elements) > 1 {
			success = rt.readRoutePair(elements[0], elements[1])
		}
		if success {
			elements = elements[2:]
		} else {
			//are there single elements we should parse?
			log.Logf("ignoring unknown symbol %s in %s", elements[0], line)
			elements = elements[1:]
		}
	}
	return true, rt
}

func (r *Route) readRoutePair(k, v string) bool {
	switch k {
	case "via":
		r.Gw = net.ParseIP(v)
	case "dev":
		r.Iface = v
	case "src":
		r.Src = net.ParseIP(v)
	case "proto":
		r.Proto = str2proto(v)
	case "metric":
		var err error
		r.Metric, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			r.Metric = 0
			log.Logf("ignoring unparsable metric %s", v)
		}
	case "scope":
		r.Scope = str2scope(v)
	case "uid":
		//ignore
	default:
		return false
	}
	return true
}

func str2scope(str string) netlink.Scope {
	var s netlink.Scope
	switch str {
	case "universe", "global":
		s = netlink.SCOPE_UNIVERSE
	case "site":
		s = netlink.SCOPE_SITE
	case "link":
		s = netlink.SCOPE_LINK
	case "host":
		s = netlink.SCOPE_HOST
	case "nowhere":
		s = netlink.SCOPE_NOWHERE
	default:
		i, err := strconv.Atoi(str)
		if err == nil {
			s = netlink.Scope(i)
		} else {
			log.Logf("unknown route scope %s", str)
		}
	}
	return s
}

func scope2str(s netlink.Scope) string {
	var str string
	switch s {
	case netlink.SCOPE_UNIVERSE:
		str = "global"
	case netlink.SCOPE_SITE:
		str = "site"
	case netlink.SCOPE_LINK:
		str = "link"
	case netlink.SCOPE_HOST:
		str = "host"
	case netlink.SCOPE_NOWHERE:
		str = "nowhere"
	default:
		str = strconv.FormatUint(uint64(s), 10)
	}
	return str
}

// Determine the WAN device.
//
// WARNING: assumes nics will not be disabled while this is running, by this or
// other process(es). Will return incorrect port as wan if that's not true.
func WanDevice(plat common.PlatInfoer) *nic.Nic {
	interfaces := nic.SortedList(plat.MACPrefixes())
	if (os.Getpid() == 1 && nic.DisabledNics() == 0) ||
		!nic.NICsAlreadyConfigured() {
		//No ports have been disabled. Filter out diag ports, if any.
		noDiag := nic.NotFilter(nic.IndexFilter(plat.DiagPorts()))
		interfaces = interfaces.Filter(noDiag)
	}
	idx := plat.WANIndex()
	if len(interfaces) > idx {
		return &interfaces[idx]
	}
	log.Logf("wan index out of range")
	return nil
}

func (r *Route) Remove() {
	del := exec.Command("ip", "route", "del", "default", "via", r.Gw.String(), "dev", r.Iface)
	out, err := del.CombinedOutput()
	if err != nil {
		log.Logf("%v: error %s\nout %s", del.Args, err, string(out))
	}
}

func (r *Route) adjustMetric(newMetric uint64, prevTries int) {
	/* 'ip route change' and 'ip route replace' exist, but do not work to update a route metric
	   (see http://lkml.iu.edu/hypermail/linux/net/0504.3/0017.html )
	   so, remove and re-add with new metric.
	*/
	if prevTries >= maxTries {
		log.Logf("too many retries adjusting route metric for '%s', giving up", r.String())
		return
	}
	r.Remove()
	adjust := exec.Command("ip", "route", "add", "default", "via", r.Gw.String(), "dev", r.Iface, "proto", proto2str(r.Proto))
	if r.Src != nil {
		adjust.Args = append(adjust.Args, "src", r.Src.String())
	}
	adjust.Args = append(adjust.Args, "metric", strconv.FormatUint(newMetric, 10))
	_, success := log.Cmd(adjust)
	if !success {
		log.Logf("failed to add route with metric %d; incrementing and retrying (%d/%d)", newMetric, prevTries+1, maxTries)
		r.adjustMetric(newMetric+1, prevTries+1)
		return
	}
	r.Metric = newMetric
}

//Renders the route in human-readable form like that of 'ip route'
func (r *Route) String() string {
	var desc string
	if r.Dest.IP == nil {
		desc = "default"
	} else {
		ones, totalBits := r.Dest.Mask.Size()
		if ones == totalBits {
			//for an ipv4 /32 or an ipv6 /128, do not include the mask
			desc = r.Dest.IP.String()
		} else {
			//ip and mask, cidr notation
			desc = r.Dest.String()
		}
	}
	if r.Gw != nil {
		desc += fmt.Sprintf(" via %s", r.Gw)
	}
	if len(r.Iface) > 0 {
		desc += " dev " + r.Iface
	}
	if r.Proto > 0 {
		desc += " proto " + proto2str(r.Proto)
	}
	if r.Scope > 0 {
		desc += " scope " + scope2str(r.Scope)
	}
	if r.Src != nil {
		desc += " src " + r.Src.String()
	}
	if r.Metric != 0 {
		desc += " metric " + strconv.FormatUint(r.Metric, 10)
	}
	return desc
}

// GetRouteThroughIface gets a route to destip through iface.
func GetRouteThroughIface(iface, destip string) Route {
	//ip r g 8.8.8.8 oif enp4s0
	//8.8.8.8 via 10.254.64.161 dev enp4s0 src 10.254.64.174 uid 1000

	iprg := exec.Command("ip", "route", "get", destip, "oif", iface)
	res, success := log.Cmd(iprg)
	r := Route{}
	if success {
		routes := parseRoutes(res, false)
		if len(routes) > 0 {
			r = routes[0]
		}
	}
	return r
}

// Add route via netlink, does not shell out to 'ip'.
func (r *Route) Add() error {
	idx, err := dev2idx(r.Iface)
	if err != nil {
		return err
	}

	nr := &netlink.Route{
		LinkIndex: idx,
		Src:       r.Src,
		Gw:        r.Gw,
		Dst:       &r.Dest.IPNet,
		Scope:     r.Scope,
		Protocol:  r.Proto,
	}
	if r.Metric > 0 {
		// The kernel has RTA_METRIC and RTA_PRIORITY; the former is a set of
		// nested attrs, and apparently the kernel derives priority from them
		// if set. github.com/vishvananda/netlink package doesn't handle these
		// nested attrs currently, and even if it did priority is simpler. It
		// seems that what iproute displays as the metric is the value the
		// kernel stores as priority.
		nr.Priority = int(r.Metric)
	}
	if err = netlink.RouteAdd(nr); err != nil {
		return fmt.Errorf("error adding route %s: %v", r.String(), err)
	}
	return nil
}

//returns an int for netlink.Protocol corresponding to input str
func str2proto(str string) int {
	switch str {
	case "kernel":
		return unix.RTPROT_KERNEL
	case "boot", "dhcp":
		return unix.RTPROT_BOOT
	case "static":
		return unix.RTPROT_STATIC
	default:
		i, err := strconv.ParseInt(str, 10, 64)
		if err == nil {
			return int(i)
		}
		log.Logf("unknown route proto %s", str)
		return unix.RTPROT_UNSPEC
	}
}
func proto2str(p int) string {
	switch p {
	case unix.RTPROT_KERNEL:
		return "kernel"
	case unix.RTPROT_BOOT:
		return "dhcp"
	case unix.RTPROT_STATIC:
		return "static"
	default:
		return strconv.FormatInt(int64(p), 10)
	}
}

//find interface index for device with given name
func dev2idx(dev string) (int, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	for _, i := range ifaces {
		if i.Name == dev {
			return i.Index, nil
		}
	}
	return 0, os.ErrInvalid
}
