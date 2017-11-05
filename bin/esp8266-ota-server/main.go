package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/msiebuhr/esp8266-ota"
	"github.com/msiebuhr/esp8266-ota/stores"
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

func fillMemoryStore(store esp8266ota.MemoryStore) {
	for i := 1; i < len(os.Args); i += 2 {
		mac, err := net.ParseMAC(os.Args[i])
		if err != nil {
			panic(err)
		}
		f, err := os.Open(os.Args[i+1])
		defer f.Close()
		if err != nil {
			panic(err)
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}
		store.AddDevice(mac, data)
	}
}

func main() {
	/*
		store := esp8266ota.NewMemoryStore()
		// Add dummy device
		mac, err := net.ParseMAC("18:FE:AA:AA:AA:AA")
		if err != nil {
			panic(err)
		}
		store.AddDevice(mac, []byte("foobar"))

		// Add other deivces from arguments w. [<MAC> <image_name> [<MAC2> <image_name2> [...]]]
		fillStoreFromArguments(store)
	*/

	store, err := stores.NewFileSystem("./data")
	if err != nil {
		panic(err)
	}

	updateHandler := esp8266ota.NewHandler(store)

	http.Handle("/get", updateHandler)

	fmt.Printf("Starting server on %s:8080\n", GetOutboundIP())
	http.ListenAndServe(":8080", nil)
}
