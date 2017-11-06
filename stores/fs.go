package stores

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/msiebuhr/httperror"
)

// FileSystem store save device-related things under
// root/devices/MAC/{request.log,sketch/active.bin}. Apps live under
// root/apps/NAME/{*.bin} where particularly active.bin should be.
//
// This way root/device/mac/sketch can be a symlink to the app the device is
// supposed to run and root/apps/NAME/active.bin a symlink to the particular
// binary the app is supposed to use.
type FileSystem struct {
	root string
}

func NewFileSystem(root string) (*FileSystem, error) {
	// TODO: Check root exists
	abspath, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// Make sure directories exist
	os.MkdirAll(filepath.Join(abspath, "devices"), 0755)
	os.MkdirAll(filepath.Join(abspath, "apps"), 0755)

	return &FileSystem{root: abspath}, nil
}

func (fs *FileSystem) LogDeviceInfo(addr net.HardwareAddr, info map[string]interface{}) error {
	// Create required directories
	infoPath := filepath.Clean(filepath.Join(fs.root, "devices", addr.String(), "info.json"))

	if !strings.HasPrefix(infoPath, fs.root) {
		return httperror.NewBadRequest("Invalid path")
	}

	err := os.MkdirAll(filepath.Dir(infoPath), 0644)
	if err != nil {
		return err
	}

	// JSON encode data
	jsonData, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := os.Create(infoPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(jsonData); err != nil {
		return err
	}
	return nil
}

func (fs *FileSystem) LogDeviceRequest(addr net.HardwareAddr, info map[string]interface{}) error {
	// Create required directories
	logPath := filepath.Clean(filepath.Join(fs.root, "devices", addr.String(), "request.log"))

	if !strings.HasPrefix(logPath, fs.root) {
		return httperror.NewBadRequest("Invalid path")
	}

	err := os.MkdirAll(filepath.Dir(logPath), 0644)
	if err != nil {
		return err
	}

	// JSON encode data
	jsonData, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(jsonData); err != nil {
		return err
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

// Return MD5-sum of the sketch device with `addr` is supposed to have
func (fs *FileSystem) GetDeviceSketchMD5(addr net.HardwareAddr) ([]byte, error) {
	data, err := fs.GetDeviceSketch(addr)
	if err != nil {
		return data, err
	}

	hash := md5.Sum(data)
	return hash[:], nil
}

func (fs *FileSystem) GetDeviceSketch(addr net.HardwareAddr) ([]byte, error) {
	// Does the device exist?
	hwPath := filepath.Clean(filepath.Join(fs.root, "devices", addr.String(), "sketch", "active.bin"))
	if !strings.HasPrefix(hwPath, fs.root) {
		fmt.Println(fs.root, hwPath)
		return []byte{}, httperror.NewBadRequest("Invalid path")
	}

	// Read directory and find link to sketch (if any)
	sketch, err := ioutil.ReadFile(hwPath)
	if err != nil {
		return []byte{}, httperror.NewNotFound("No sketch available")
	}

	return sketch, nil
}
