package gears

import (
	"sort"
	"strings"

	"github.com/giztoy/giztoy-go/pkg/kv"
)

var gearsRoot = kv.Key{"gears"}

func gearKey(publicKey string) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-pubkey", publicKey)
}

func gearsPrefix() kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-pubkey")
}

func snKey(sn string) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-sn", escapeIndexSegment(sn))
}

func imeiKey(tac, serial string) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-imei", escapeIndexSegment(tac), escapeIndexSegment(serial))
}

func imeiTACPrefix(tac string) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-imei-tac", escapeIndexSegment(tac))
}

func imeiTACKey(tac, publicKey string) kv.Key {
	return append(imeiTACPrefix(tac), publicKey)
}

func certificationPrefix(cert GearCertification) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-certification", string(cert.Type), string(cert.Authority), escapeIndexSegment(cert.ID))
}

func certificationKey(cert GearCertification, publicKey string) kv.Key {
	return append(certificationPrefix(cert), publicKey)
}

func labelPrefix(label GearLabel) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-label", escapeIndexSegment(label.Key), escapeIndexSegment(label.Value))
}

func labelKey(label GearLabel, publicKey string) kv.Key {
	return append(labelPrefix(label), publicKey)
}

func firmwarePrefix(depot string, channel GearFirmwareChannel) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-firmware-depot", escapeIndexSegment(depot), string(channel))
}

func firmwareKey(depot string, channel GearFirmwareChannel, publicKey string) kv.Key {
	return append(firmwarePrefix(depot, channel), publicKey)
}

func rolePrefix(role GearRole) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-role", string(role))
}

func roleKey(role GearRole, publicKey string) kv.Key {
	return append(rolePrefix(role), publicKey)
}

func statusPrefix(status GearStatus) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-status", string(status))
}

func statusKey(status GearStatus, publicKey string) kv.Key {
	return append(statusPrefix(status), publicKey)
}

type indexSnapshot struct {
	sn             string
	imeis          []GearIMEI
	certifications []GearCertification
	labels         []GearLabel
	depot          string
	channel        GearFirmwareChannel
	role           GearRole
	status         GearStatus
}

func snapshotIndexes(g Gear) indexSnapshot {
	return indexSnapshot{
		sn:             g.Device.SN,
		imeis:          dedupeIMEIs(g.Device.Hardware.IMEIs),
		certifications: dedupeCertifications(g.Configuration.Certifications),
		labels:         dedupeLabels(g.Device.Hardware.Labels),
		depot:          g.Device.Hardware.Depot,
		channel:        g.Configuration.Firmware.Channel,
		role:           g.Role,
		status:         g.Status,
	}
}

func dedupeIMEIs(items []GearIMEI) []GearIMEI {
	seen := make(map[[2]string]struct{}, len(items))
	out := make([]GearIMEI, 0, len(items))
	for _, item := range items {
		if item.TAC == "" || item.Serial == "" {
			continue
		}
		key := [2]string{item.TAC, item.Serial}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TAC == out[j].TAC {
			return out[i].Serial < out[j].Serial
		}
		return out[i].TAC < out[j].TAC
	})
	return out
}

func dedupeLabels(items []GearLabel) []GearLabel {
	seen := make(map[[2]string]struct{}, len(items))
	out := make([]GearLabel, 0, len(items))
	for _, item := range items {
		if item.Key == "" || item.Value == "" {
			continue
		}
		key := [2]string{item.Key, item.Value}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key == out[j].Key {
			return out[i].Value < out[j].Value
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func dedupeCertifications(items []GearCertification) []GearCertification {
	seen := make(map[[3]string]struct{}, len(items))
	out := make([]GearCertification, 0, len(items))
	for _, item := range items {
		if item.Type == "" || item.Authority == "" || item.ID == "" {
			continue
		}
		key := [3]string{string(item.Type), string(item.Authority), item.ID}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			if out[i].Authority == out[j].Authority {
				return out[i].ID < out[j].ID
			}
			return out[i].Authority < out[j].Authority
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func escapeIndexSegment(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	return strings.ReplaceAll(value, ":", "%3A")
}
