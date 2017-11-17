package stores

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// Creates the needed directories for the device and retunrs the base
// string.
func (fs FileSystem) ensureDeviceExist(addr net.HardwareAddr) (string, error) {
	devicePath := filepath.Clean(filepath.Join(fs.root, "devices", addr.String()))

	if !strings.HasPrefix(devicePath, fs.root) {
		return "", httperror.NewBadRequest("Invalid path")
	}

	err := os.MkdirAll(devicePath, 0755)
	return devicePath, err
}
func (fs FileSystem) ensureAppExist(name string) (string, error) {
	appPath := filepath.Clean(filepath.Join(fs.root, "apps", name))

	if !strings.HasPrefix(appPath, fs.root) {
		return "", httperror.NewBadRequest("Invalid path")
	}

	err := os.MkdirAll(appPath, 0755)
	return appPath, err
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
	deviceDir, err := fs.ensureDeviceExist(addr)
	if err != nil {
		return err
	}

	// Create required directories
	infoPath := filepath.Join(deviceDir, "info.json")

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
	deviceDir, err := fs.ensureDeviceExist(addr)
	if err != nil {
		return err
	}

	// Create required directories
	logPath := filepath.Join(deviceDir, "request.log")

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
		return []byte{}, httperror.NewBadRequest("Invalid path")
	}

	// Read directory and find link to sketch (if any)
	sketch, err := ioutil.ReadFile(hwPath)
	if err != nil {
		return []byte{}, httperror.NewNotFound("No sketch available")
	}

	return sketch, nil
}

// Simple-ish web interface
func (fs *FileSystem) CreateApp(app string) error {
	_, err := fs.ensureAppExist(app)
	return err
}

func (fs *FileSystem) UploadAppSketch(app, sketchName string, sketch []byte) error {
	dir, err := fs.ensureAppExist(app)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(".bin", sketchName) {
		sketchName = sketchName + ".bin"
	}

	return ioutil.WriteFile(filepath.Join(dir, sketchName), sketch, 0644)
}

func (fs *FileSystem) SetActiveAppSketch(app, sketchName string) error {
	// TODO(mortens): Make sure target exist
	// TODO(mortens): Don't bother if change is NO-OP
	dir, err := fs.ensureAppExist(app)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(sketchName, ".bin") {
		sketchName = sketchName + ".bin"
	}

	// Need to remove active.bin first...
	os.Remove(filepath.Join(dir, "active.bin"))

	return os.Symlink(sketchName, filepath.Join(dir, "active.bin"))
}

func (fs *FileSystem) DeviceSetApp(addr net.HardwareAddr, app string) error {
	devicePath, err := fs.ensureDeviceExist(addr)
	if err != nil {
		return err
	}

	appPath, err := fs.ensureAppExist(app)
	if err != nil {
		return err
	}

	// Need to remove active.bin first...
	os.Remove(filepath.Join(devicePath, "sketch"))

	// ln -s devicePath/sketch -> appPath (relative)
	relPath, err := filepath.Rel(devicePath, appPath)
	if err != nil {
		return err
	}

	return os.Symlink(relPath, filepath.Join(devicePath, "sketch"))
}

// Quick handler that exposes admin interface

func unmarshalJSONBody(req *http.Request, v interface{}) error {
	defer req.Body.Close()

	// Read and parse JSON body
	raw, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(raw, v)
}

func (fs *FileSystem) GetAdminMux() http.Handler {
	mux := http.NewServeMux()

	// Some handy structs
	type appSketch struct {
		Name    string
		Size    int64
		ModTime time.Time
		// Only used when uploading new sketches
		Data []byte
	}

	type app struct {
		Name         string
		ActiveSketch string
		Sketches     []appSketch
	}

	type device struct {
		Name    string
		Info    map[string]interface{}
		AppName string
	}

	// List devices
	// TODO: Should probably be API call...
	mux.HandleFunc("/devices", func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		dir, err := os.Open(filepath.Join(fs.root, "devices"))
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		// List directories
		entries, err := dir.Readdirnames(0)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		data := make([]device, len(entries))

		// TODO: Last seen, general info &c.
		for i, entry := range entries {
			data[i].Name = entry

			baseDir := filepath.Join(fs.root, "devices", entry)
			// Read info for file
			info, _ := ioutil.ReadFile(
				filepath.Join(baseDir, "info.json"),
			)

			err := json.Unmarshal(info, &data[i].Info)
			if err != nil {
				data[i].Info = map[string]interface{}{"err": err}
			}

			// Read active sketch
			target, err := os.Readlink(filepath.Join(baseDir, "sketch"))
			if err == nil {
				data[i].AppName = filepath.Base(target)
			}
		}

		j, err := json.Marshal(data)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		w.Write(j)
	})

	mux.HandleFunc("/device/set-app", func(w http.ResponseWriter, req *http.Request) {
		data := &device{}
		err := unmarshalJSONBody(req, data)
		if err != nil {
			httperror.WrapBadRequest(err).Respond(w)
			return
		}

		if data.Name == "" || data.AppName == "" {
			httperror.NewBadRequest("Missing parameter `Name` or `AppName`").Respond(w)
			return
		}

		mac, err := net.ParseMAC(data.Name)
		if err != nil {
			httperror.WrapBadRequest(err).Respond(w)
			return
		}

		err = fs.DeviceSetApp(mac, data.AppName)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		httperror.NewOK().Respond(w)
	})

	mux.HandleFunc("/apps", func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()

		dir, err := os.Open(filepath.Join(fs.root, "apps"))
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}
		defer dir.Close()

		// List directories
		entries, err := dir.Readdirnames(0)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		// Loop over apps
		data := make([]app, len(entries))
		for i, entry := range entries {
			data[i].Name = entry

			// Read active sketch
			target, err := os.Readlink(filepath.Join(fs.root, "apps", entry, "active.bin"))
			if err == nil {
				data[i].ActiveSketch = filepath.Base(target)
			}

			// Open folder in question
			subdir, err := os.Open(filepath.Join(fs.root, "apps", entry))
			if err != nil {
				httperror.WrapInternalServerError(err).Respond(w)
				return
			}

			// List sketches
			sketches, err := subdir.Readdir(0)
			if err != nil {
				httperror.WrapInternalServerError(err).Respond(w)
				return
			}
			data[i].Sketches = make([]appSketch, 0, 0)

			for _, sketchLstat := range sketches {
				if sketchLstat.Name() == "active.bin" {
					continue
				}
				data[i].Sketches = append(data[i].Sketches, appSketch{
					Name:    sketchLstat.Name(),
					Size:    sketchLstat.Size(),
					ModTime: sketchLstat.ModTime().UTC().Round(1 * time.Second),
				})
			}
		}

		j, err := json.Marshal(data)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		w.Write(j)
	})

	mux.HandleFunc("/apps/new", func(w http.ResponseWriter, req *http.Request) {
		data := &app{}
		err := unmarshalJSONBody(req, data)
		if err != nil {
			httperror.WrapBadRequest(err).Respond(w)
			return
		}

		if data.Name == "" {
			httperror.NewBadRequest("Missing parameter `Name` or `ActiveSketch`").Respond(w)
			return
		}

		err = fs.CreateApp(data.Name)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		httperror.NewOK().Respond(w)
	})

	mux.HandleFunc("/apps/set-sketch", func(w http.ResponseWriter, req *http.Request) {
		data := &app{}
		err := unmarshalJSONBody(req, data)
		if err != nil {
			httperror.WrapBadRequest(err).Respond(w)
			return
		}

		if data.Name == "" || data.ActiveSketch == "" {
			httperror.NewBadRequest("Missing parameter `Name` or `ActiveSketch`").Respond(w)
			return
		}

		err = fs.SetActiveAppSketch(data.Name, data.ActiveSketch)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		httperror.NewOK().Respond(w)
	})

	mux.HandleFunc("/apps/add-sketch", func(w http.ResponseWriter, req *http.Request) {
		data := &app{}
		err := unmarshalJSONBody(req, data)
		if err != nil {
			httperror.WrapBadRequest(err).Respond(w)
			return
		}

		if data.Name == "" || len(data.Sketches) == 0 || data.Sketches[0].Name == "" || len(data.Sketches[0].Data) == 0 {
			httperror.NewBadRequest("Missing parameter `Name` or `Sketches`").Respond(w)
			return
		}

		if !bytes.HasPrefix(data.Sketches[0].Data, []byte{0xe9, 0x01, 0x02, 0x40, 0x9c, 0xf2, 0x10, 0x40}) {
			httperror.NewBadRequest("File has unexpected magic numbers").Respond(w)
			return
		}

		err = fs.UploadAppSketch(data.Name, data.Sketches[0].Name, data.Sketches[0].Data)
		if err != nil {
			httperror.WrapInternalServerError(err).Respond(w)
			return
		}

		httperror.NewOK().Respond(w)
	})

	mux.Handle("/", http.FileServer(http.Dir("./stores/fs-static")))

	return mux
}
