package gear

import (
	"sort"
	"strings"

	"github.com/GizClaw/gizclaw-go/pkg/gizclaw/api/apitypes"

	"github.com/GizClaw/gizclaw-go/pkg/store/kv"
)

func firmwareChannel(cfg apitypes.Configuration) string {
	if cfg.Firmware == nil || cfg.Firmware.Channel == nil {
		return ""
	}
	return string(*cfg.Firmware.Channel)
}

func gearSN(gear apitypes.Gear) string {
	if gear.Device.Sn == nil {
		return ""
	}
	return *gear.Device.Sn
}

func gearDepot(gear apitypes.Gear) string {
	if gear.Device.Hardware == nil || gear.Device.Hardware.Depot == nil {
		return ""
	}
	return *gear.Device.Hardware.Depot
}

func gearIMEIs(gear apitypes.Gear) []apitypes.GearIMEI {
	if gear.Device.Hardware == nil || gear.Device.Hardware.Imeis == nil {
		return nil
	}
	return *gear.Device.Hardware.Imeis
}

func gearLabels(gear apitypes.Gear) []apitypes.GearLabel {
	if gear.Device.Hardware == nil || gear.Device.Hardware.Labels == nil {
		return nil
	}
	return *gear.Device.Hardware.Labels
}

func gearCertifications(gear apitypes.Gear) []apitypes.GearCertification {
	if gear.Configuration.Certifications == nil {
		return nil
	}
	return *gear.Configuration.Certifications
}

func dedupeIMEIs(items []apitypes.GearIMEI) []apitypes.GearIMEI {
	seen := make(map[[2]string]struct{}, len(items))
	out := make([]apitypes.GearIMEI, 0, len(items))
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

func dedupeLabels(items []apitypes.GearLabel) []apitypes.GearLabel {
	seen := make(map[[2]string]struct{}, len(items))
	out := make([]apitypes.GearLabel, 0, len(items))
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

func dedupeCertifications(items []apitypes.GearCertification) []apitypes.GearCertification {
	seen := make(map[[3]string]struct{}, len(items))
	out := make([]apitypes.GearCertification, 0, len(items))
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

func gearKey(publicKey string) kv.Key {
	return kv.Key{"by-pubkey", publicKey}
}

func gearsPrefix() kv.Key {
	return kv.Key{"by-pubkey"}
}

func snKey(sn string) kv.Key {
	return kv.Key{"by-sn", escapeIndexSegment(sn)}
}

func imeiKey(tac, serial string) kv.Key {
	return kv.Key{"by-imei", escapeIndexSegment(tac), escapeIndexSegment(serial)}
}

func certificationPrefix(certType apitypes.GearCertificationType, authority apitypes.GearCertificationAuthority, id string) kv.Key {
	return kv.Key{"by-certification", string(certType), string(authority), escapeIndexSegment(id)}
}

func certificationKey(item apitypes.GearCertification, publicKey string) kv.Key {
	return append(certificationPrefix(item.Type, item.Authority, item.Id), publicKey)
}

func labelPrefix(key, value string) kv.Key {
	return kv.Key{"by-label", escapeIndexSegment(key), escapeIndexSegment(value)}
}

func labelKey(item apitypes.GearLabel, publicKey string) kv.Key {
	return append(labelPrefix(item.Key, item.Value), publicKey)
}

func firmwarePrefix(depot string, channel apitypes.GearFirmwareChannel) kv.Key {
	return kv.Key{"by-firmware-depot", escapeIndexSegment(depot), string(channel)}
}

func firmwareKey(depot string, channel apitypes.GearFirmwareChannel, publicKey string) kv.Key {
	return append(firmwarePrefix(depot, channel), publicKey)
}

func rolePrefix(role apitypes.GearRole) kv.Key {
	return kv.Key{"by-role", string(role)}
}

func roleKey(role apitypes.GearRole, publicKey string) kv.Key {
	return append(rolePrefix(role), publicKey)
}

func statusPrefix(status apitypes.GearStatus) kv.Key {
	return kv.Key{"by-status", string(status)}
}

func statusKey(status apitypes.GearStatus, publicKey string) kv.Key {
	return append(statusPrefix(status), publicKey)
}

func indexEntries(gear apitypes.Gear) []kv.Entry {
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

func indexKeys(gear apitypes.Gear) []kv.Key {
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
