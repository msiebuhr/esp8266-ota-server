package esp8266ota

import (
	"bytes"
	"crypto/md5"
	"io"
	"net"
)

// Stores allow for accessing device-specific information and information
// relevant for updating the sketch.
//
// TODO(msiebuhr): If the OTA-library allow updating SPIFFS data, we
// should have calls for that too. Or perhaps another interface, so
// there can be per-device spiffs images...
//
// TODO(msiebuhr): What about unknown devices?

type Store interface {
	//SetDeviceInfo(net.HardwareAddr, map[string]string)
	GetDeviceInfo(net.HardwareAddr) map[string]string
	GetDeviceSketchMD5(net.HardwareAddr) []byte
	GetDeviceSketch(net.HardwareAddr) io.Reader
}

// MemoryStore is a Store-implementation in memory.
type memoryStoreDevice struct {
	info   map[string]string
	sketch []byte
}
type MemoryStore map[string]*memoryStoreDevice

func NewMemoryStore() MemoryStore {
	return MemoryStore(make(map[string]*memoryStoreDevice, 1))
}

func (ms MemoryStore) AddDevice(addr net.HardwareAddr, sketch []byte) {
	ms[addr.String()] = &memoryStoreDevice{
		info:   make(map[string]string),
		sketch: sketch,
	}
}

func (ms MemoryStore) GetDeviceInfo(addr net.HardwareAddr) map[string]string {
	if data, ok := ms[addr.String()]; ok {
		return data.info
	}

	// TODO: Something better than an empty map...
	return make(map[string]string, 0)
}

func (ms MemoryStore) GetDeviceSketch(addr net.HardwareAddr) io.Reader {
	if data, ok := ms[addr.String()]; ok {
		return bytes.NewReader(data.sketch)
	}
	return bytes.NewReader([]byte{})
}

func (ms MemoryStore) GetDeviceSketchMD5(addr net.HardwareAddr) []byte {
	if data, ok := ms[addr.String()]; ok {
		hash := md5.Sum(data.sketch)
		return hash[:]

	}
	return []byte{} // Zero array!?
}
