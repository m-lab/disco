package archive

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/m-lab/go/rtx"
)

// Sample represents the basic structure for metric samples.
type Sample struct {
	Timestamp int64  `json:"timestamp"`
	Value     uint64 `json:"value"`
}

// Model represents the structure of metric for DISCO.
type Model struct {
	Experiment string   `json:"experiment"`
	Hostname   string   `json:"hostname"`
	Metric     string   `json:"metric"`
	Samples    []Sample `json:"sample"`
}

// GetJSON accepts a Model object and returns marshalled JSON.
func GetJSON(m Model) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "    ")
	rtx.Must(err, "ERROR: failed to marshal archive.Model to JSON. This should never happen")
	return data, err
}

// GetPath returns a relative filesystem path where an archive should be written.
func GetPath(now time.Time, hostname string, interval uint64) string {
	// The directory path where the archive should be written.
	dirs := fmt.Sprintf("%v/%v", now.Format("2006/01/02"), hostname)

	// Calculate the start time, which will be Now() - interval, and then format
	// the archive file name based on the calculated values.
	startTime := now.Add(time.Duration(interval) * -time.Second)
	startTimeStr := startTime.Format("2006-01-02T15:04:05")
	endTimeStr := now.Format("2006-01-02T15:04:05")
	archiveName := fmt.Sprintf("%v-to-%v-switch.json", startTimeStr, endTimeStr)
	archivePath := fmt.Sprintf("%v/%v", dirs, archiveName)

	return archivePath
}

// Write writes out JSON data to a file on disk.
func Write(archivePath string, data []byte) error {
	dirPath := path.Dir(archivePath)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		log.Printf("ERROR: failed to create archive directory path '%v': %v", dirPath, err)
		return err
	}

	err = ioutil.WriteFile(archivePath, data, 0644)
	rtx.Must(err, "ERROR: failed to write archive file. This should never happen.")

	return nil
}
