package esp8266ota

import (
	"crypto/md5"
	"net"

	"github.com/msiebuhr/httperror"
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
	LogDeviceInfo(net.HardwareAddr, map[string]interface{}) error
	LogDeviceRequest(net.HardwareAddr, map[string]interface{}) error
	GetDeviceSketchMD5(net.HardwareAddr) ([]byte, error)
	GetDeviceSketch(net.HardwareAddr) ([]byte, error)
}

// MemoryStore is a Store-implementation in memory.
type memoryStoreDevice struct {
	info     map[string]interface{}
	requests []map[string]interface{}
	sketch   []byte
}
type MemoryStore map[string]*memoryStoreDevice

func NewMemoryStore() MemoryStore {
	return MemoryStore(make(map[string]*memoryStoreDevice, 1))
}

func (ms MemoryStore) AddDevice(addr net.HardwareAddr, sketch []byte) {
	ms[addr.String()] = &memoryStoreDevice{
		info:     make(map[string]interface{}),
		requests: make([]map[string]interface{}, 0),
		sketch:   sketch,
	}
}

func (ms MemoryStore) LogDeviceInfo(addr net.HardwareAddr, info map[string]interface{}) error {
	if data, ok := ms[addr.String()]; ok {
		data.info = info
		return nil
	}

	ms[addr.String()] = &memoryStoreDevice{
		info:   info,
		sketch: []byte{},
	}

	return nil
}

func (ms MemoryStore) LogDeviceRequest(addr net.HardwareAddr, info map[string]interface{}) error {
	if data, ok := ms[addr.String()]; ok {
		data.requests = append(data.requests, info)
		return nil
	}

	ms[addr.String()] = &memoryStoreDevice{
	//info: make(map[string]interface{})
	//requests: []map[string]interface{}{info}
	//sketch: []byte{},
	}

	return nil
}

func (ms MemoryStore) GetDeviceSketch(addr net.HardwareAddr) ([]byte, error) {
	if data, ok := ms[addr.String()]; ok {
		return data.sketch, nil
	}
	return nil, httperror.NewNotFound()
}

func (ms MemoryStore) GetDeviceSketchMD5(addr net.HardwareAddr) ([]byte, error) {
	if data, ok := ms[addr.String()]; ok {
		hash := md5.Sum(data.sketch)
		return hash[:], nil

	}
	return nil, httperror.NewNotFound()
}
