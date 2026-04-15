package gear

import (
	"sort"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/gearservice"
	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func firmwareChannel(cfg gearservice.Configuration) string {
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil {
		return ""
	}
	return string(*cfg.Firmware.Channel)
}

func gearSN(gear gearservice.Gear) string {
	if gear.Device.Sn == nil {
		return ""
	}
	return *gear.Device.Sn
}

func gearDepot(gear gearservice.Gear) string {
	if gear.Device.Hardware == nil || gear.Device.Hardware.Depot == nil {
		return ""
	}
	return *gear.Device.Hardware.Depot
}

func gearIMEIs(gear gearservice.Gear) []gearservice.GearIMEI {
	if gear.Device.Hardware == nil || gear.Device.Hardware.Imeis == nil {
		return nil
	}
	return *gear.Device.Hardware.Imeis
}

func gearLabels(gear gearservice.Gear) []gearservice.GearLabel {
	if gear.Device.Hardware == nil || gear.Device.Hardware.Labels == nil {
		return nil
	}
	return *gear.Device.Hardware.Labels
}

func gearCertifications(gear gearservice.Gear) []gearservice.GearCertification {
	if gear.Configuration.Certifications == nil {
		return nil
	}
	return *gear.Configuration.Certifications
}

func dedupeIMEIs(items []gearservice.GearIMEI) []gearservice.GearIMEI {
	seen := make(map[[2]string]struct{}, len(items))
	out := make([]gearservice.GearIMEI, 0, len(items))
	for _, item := range items {
		if item.Tac == "" || item.Serial == "" {
			continue
		}
		key := [2]string{item.Tac, item.Serial}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tac == out[j].Tac {
			return out[i].Serial < out[j].Serial
		}
		return out[i].Tac < out[j].Tac
	})
	return out
}

func dedupeLabels(items []gearservice.GearLabel) []gearservice.GearLabel {
	seen := make(map[[2]string]struct{}, len(items))
	out := make([]gearservice.GearLabel, 0, len(items))
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

func dedupeCertifications(items []gearservice.GearCertification) []gearservice.GearCertification {
	seen := make(map[[3]string]struct{}, len(items))
	out := make([]gearservice.GearCertification, 0, len(items))
	for _, item := range items {
		if item.Type == "" || item.Authority == "" || item.Id == "" {
			continue
		}
		key := [3]string{string(item.Type), string(item.Authority), item.Id}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			if out[i].Authority == out[j].Authority {
				return out[i].Id < out[j].Id
			}
			return out[i].Authority < out[j].Authority
		}
		return out[i].Type < out[j].Type
	})
	return out
}

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

func certificationPrefix(certType gearservice.GearCertificationType, authority gearservice.GearCertificationAuthority, id string) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-certification", string(certType), string(authority), escapeIndexSegment(id))
}

func certificationKey(item gearservice.GearCertification, publicKey string) kv.Key {
	return append(certificationPrefix(item.Type, item.Authority, item.Id), publicKey)
}

func labelPrefix(key, value string) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-label", escapeIndexSegment(key), escapeIndexSegment(value))
}

func labelKey(item gearservice.GearLabel, publicKey string) kv.Key {
	return append(labelPrefix(item.Key, item.Value), publicKey)
}

func firmwarePrefix(depot string, channel gearservice.GearFirmwareChannel) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-firmware-depot", escapeIndexSegment(depot), string(channel))
}

func firmwareKey(depot string, channel gearservice.GearFirmwareChannel, publicKey string) kv.Key {
	return append(firmwarePrefix(depot, channel), publicKey)
}

func rolePrefix(role gearservice.GearRole) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-role", string(role))
}

func roleKey(role gearservice.GearRole, publicKey string) kv.Key {
	return append(rolePrefix(role), publicKey)
}

func statusPrefix(status gearservice.GearStatus) kv.Key {
	return append(append(kv.Key{}, gearsRoot...), "by-status", string(status))
}

func statusKey(status gearservice.GearStatus, publicKey string) kv.Key {
	return append(statusPrefix(status), publicKey)
}

func indexEntries(gear gearservice.Gear) []kv.Entry {
	publicKey := gear.PublicKey
	entries := make([]kv.Entry, 0, 2+len(gearIMEIs(gear))+len(gearCertifications(gear))+len(gearLabels(gear)))
	if sn := gearSN(gear); sn != "" {
		entries = append(entries, kv.Entry{Key: snKey(sn), Value: []byte(publicKey)})
	}
	for _, item := range dedupeIMEIs(gearIMEIs(gear)) {
		entries = append(entries, kv.Entry{Key: imeiKey(item.Tac, item.Serial), Value: []byte(publicKey)})
	}
	for _, item := range dedupeCertifications(gearCertifications(gear)) {
		entries = append(entries, kv.Entry{Key: certificationKey(item, publicKey), Value: []byte{1}})
	}
	for _, item := range dedupeLabels(gearLabels(gear)) {
		entries = append(entries, kv.Entry{Key: labelKey(item, publicKey), Value: []byte{1}})
	}
	if depot := gearDepot(gear); depot != "" && firmwareChannel(gear.Configuration) != "" {
		entries = append(entries, kv.Entry{
			Key:   firmwareKey(depot, *gear.Configuration.Firmware.Channel, publicKey),
			Value: []byte{1},
		})
	}
	if gear.Role != "" {
		entries = append(entries, kv.Entry{Key: roleKey(gear.Role, publicKey), Value: []byte{1}})
	}
	if gear.Status != "" {
		entries = append(entries, kv.Entry{Key: statusKey(gear.Status, publicKey), Value: []byte{1}})
	}
	return entries
}

func indexKeys(gear gearservice.Gear) []kv.Key {
	publicKey := gear.PublicKey
	keys := make([]kv.Key, 0, 2+len(gearIMEIs(gear))+len(gearCertifications(gear))+len(gearLabels(gear)))
	if sn := gearSN(gear); sn != "" {
		keys = append(keys, snKey(sn))
	}
	for _, item := range dedupeIMEIs(gearIMEIs(gear)) {
		keys = append(keys, imeiKey(item.Tac, item.Serial))
	}
	for _, item := range dedupeCertifications(gearCertifications(gear)) {
		keys = append(keys, certificationKey(item, publicKey))
	}
	for _, item := range dedupeLabels(gearLabels(gear)) {
		keys = append(keys, labelKey(item, publicKey))
	}
	if depot := gearDepot(gear); depot != "" && firmwareChannel(gear.Configuration) != "" {
		keys = append(keys, firmwareKey(depot, *gear.Configuration.Firmware.Channel, publicKey))
	}
	if gear.Role != "" {
		keys = append(keys, roleKey(gear.Role, publicKey))
	}
	if gear.Status != "" {
		keys = append(keys, statusKey(gear.Status, publicKey))
	}
	return keys
}

func escapeIndexSegment(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	return strings.ReplaceAll(value, ":", "%3A")
}
