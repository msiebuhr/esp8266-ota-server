package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/msiebuhr/esp8266-ota"
)

// Test using
//
//    curl localhost:8080/ -H 'x-ESP8266-STA-MAC: 18:FE:AA:AA:AA:AA' -H 'x-ESP8266-sketch-md5: `echo -n foobar | md5`' -v

// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func main() {
	store := esp8266ota.NewMemoryStore()

	mac, err := net.ParseMAC("18:FE:AA:AA:AA:AA")

	if err != nil {
		panic(err)
	}

	store.AddDevice(mac, []byte("foobar"))

	updateHandler := esp8266ota.NewHandler(store)

	http.Handle("/", updateHandler)

	fmt.Printf("Starting server on %s:8080", GetOutboundIP())
	http.ListenAndServe(":8080", nil)
}
