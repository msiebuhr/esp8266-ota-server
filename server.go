package esp8266ota

// This package implements an Over-the-air (OTA) update server for ESP8266.
//
// See https://esp8266.github.io/Arduino/versions/2.0.0/doc/ota_updates/ota_updates.html for details
//
// Client implementation at https://github.com/esp8266/Arduino/blob/master/libraries/ESP8266httpUpdate/src/ESP8266httpUpdate.cpp#L174

import (
	"bytes"
	"encoding/hex"
	"io"
	"net"
	"net/http"
)

type Handler struct {
	store Store
}

func NewHandler(s Store) Handler {
	return Handler{
		store: s,
	}
}

func (s Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.Body.Close() // We're not gonna use that

	macStrs, ok := req.Header["X-Esp8266-Sta-Mac"] // Correct name?
	if !ok || len(macStrs) != 1 {
		http.Error(w, "One `X-ESP8266-STA-MAC` header required", http.StatusBadRequest)
		return
	}
	mac, _ := net.ParseMAC(macStrs[0])

	md5Strs, ok := req.Header["X-Esp8266-Sketch-Md5"] // Corerct one?
	if !ok || len(md5Strs) != 1 {
		http.Error(w, "One `X-ESP8266-sketch-md5` header required", http.StatusBadRequest)
		return
	}
	md5, err := hex.DecodeString(md5Strs[0])
	if err != nil {
		panic(err)
	}

	// TODO: Update device's info with new data
	//info = s.store.GetDeviceInfo(mac);

	desiredMD5 := s.store.GetDeviceSketchMD5(mac)

	if bytes.Equal(desiredMD5, []byte{}) {
		http.Error(w, "Unknown device", http.StatusBadRequest)
		return
	}

	// If the received MD5 matches the stored one, do nothing
	if bytes.Equal(desiredMD5, md5) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Update 'a-comin
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename='sketch.bin'")
	//w.Header().Set("Content-Length", <How much GetDeviceSketch() returned)
	w.Header().Set("x-MD5", string(desiredMD5))

	io.Copy(w, s.store.GetDeviceSketch(mac))
}
