package esp8266ota

// This package implements an Over-the-air (OTA) update server for ESP8266.
//
// See https://esp8266.github.io/Arduino/versions/2.0.0/doc/ota_updates/ota_updates.html for details
//
// Client implementation at https://github.com/esp8266/Arduino/blob/master/libraries/ESP8266httpUpdate/src/ESP8266httpUpdate.cpp#L174

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/msiebuhr/httperror"
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
	fmt.Println("Got MAC", mac, "md5", hex.EncodeToString(md5))

	// TODO: Update device's info with new data
	//info = s.store.GetDeviceInfo(mac);

	info := map[string]interface{}{}
	for name, values := range req.Header {
		if strings.HasPrefix(name, "X-Esp8266-") {
			info[name[len("X-Esp8266-"):]] = values[0]
		}
	}
	s.store.LogDeviceRequest(mac, info)

	desiredMD5, err := s.store.GetDeviceSketchMD5(mac)
	if err != nil {
		if herr, ok := httperror.IsHTTPError(err); ok {
			herr.Respond(w)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if bytes.Equal(desiredMD5, []byte{}) {
		http.Error(w, "Unknown device", http.StatusBadRequest)
		return
	}

	// If the received MD5 matches the stored one, do nothing
	if bytes.Equal(desiredMD5, md5) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	sketch, err := s.store.GetDeviceSketch(mac)
	if err != nil {
		http.Error(w, "Could not fetch sketch", http.StatusInternalServerError)
		return
	}

	// Update 'a-comin
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(sketch)))
	//w.Header().Set("Content-Disposition", "attachment; filename='sketch.bin'")
	//w.Header().Set("x-MD5", hex.EncodeToString(desiredMD5))
	// Write `x-MD5`-header without having it passed through normalization
	w.Header()["x-MD5"] = []string{hex.EncodeToString(desiredMD5)}
	w.WriteHeader(http.StatusOK)

	count, err := w.Write(sketch)
	fmt.Println("Wrote", count, err)
	req.Body.Close() // We're not gonna use that
}
