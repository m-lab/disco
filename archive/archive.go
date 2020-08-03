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
	Timestamp    int64  `json:"timestamp"`
	CollectStart int64  `json:"collectstart"`
	CollectEnd   int64  `json:"collectend"`
	Value        uint64 `json:"value"`
}

// Model represents the structure of metric for DISCO.
type Model struct {
	Experiment string   `json:"experiment"`
	Hostname   string   `json:"hostname"`
	Metric     string   `json:"metric"`
	Samples    []Sample `json:"sample"`
}

// MustMarshalJSON accepts a Model object and returns marshalled JSON.
func MustMarshalJSON(m Model) []byte {
	data, err := json.Marshal(m)
	rtx.Must(err, "ERROR: failed to marshal archive.Model to JSON. This should never happen")
	return data
}

// GetPath returns a filesystem path where an archive should be written.
func GetPath(start time.Time, end time.Time, dataDir string, hostname string) string {
	// The directory path where the archive should be written.
	dirs := fmt.Sprintf("%v/%v", end.Format("2006/01/02"), hostname)

	startTimeStr := start.Format("2006-01-02T15:04:05")
	endTimeStr := end.Format("2006-01-02T15:04:05")
	archiveName := fmt.Sprintf("%v-to-%v-switch.jsonl", startTimeStr, endTimeStr)
	archivePath := fmt.Sprintf("%v/switch/%v/%v", dataDir, dirs, archiveName)

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
