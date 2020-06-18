package archive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
)

var testModels = []Model{
	Model{
		Experiment: "s1-abc0t.measurement-lab.org",
		Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
		Metric:     "switch.unicast.uplink.tx",
		Samples: []Sample{
			Sample{
				Timestamp:    1591845348,
				CollectStart: 1592344911240000000,
				CollectEnd:   1592344912360000000,
				Value:        158,
			},
			Sample{
				Timestamp:    1591845358,
				CollectStart: 1592344911240000000,
				CollectEnd:   1592344912360000000,
				Value:        132,
			},
		},
	},
	Model{
		Experiment: "s1-abc0t.measurement-lab.org",
		Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
		Metric:     "switch.octets.local.rx",
		Samples: []Sample{
			Sample{
				Timestamp:    1591845348,
				CollectStart: 1592344911240000000,
				CollectEnd:   1592344912360000000,
				Value:        3256,
			},
			Sample{
				Timestamp:    1591845358,
				CollectStart: 1592344911240000000,
				CollectEnd:   1592344912360000000,
				Value:        2789,
			},
		},
	},
}

func Test_GetPath(t *testing.T) {
	tests := []struct {
		end      time.Time
		interval uint64
		hostname string
		expect   string
	}{
		{
			end:      time.Date(2010, 04, 18, 20, 34, 50, 0, time.UTC),
			interval: 60,
			hostname: "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
			expect:   "2010/04/18/mlab2-abc0t.mlab-sandbox.measurement-lab.org/2010-04-18T20:33:50-to-2010-04-18T20:34:50-switch.jsonl",
		},
		{
			end:      time.Date(1972, 07, 03, 11, 14, 10, 0, time.UTC),
			interval: 600,
			hostname: "mlab4-xyz03.mlab-staging.measurement-lab.org",
			expect:   "1972/07/03/mlab4-xyz03.mlab-staging.measurement-lab.org/1972-07-03T11:04:10-to-1972-07-03T11:14:10-switch.jsonl",
		},
		{
			end:      time.Date(2020, 06, 11, 18, 18, 30, 0, time.UTC),
			interval: 300,
			hostname: "mlab1-qrs0t.mlab-sandbox.measurement-lab.org",
			expect:   "2020/06/11/mlab1-qrs0t.mlab-sandbox.measurement-lab.org/2020-06-11T18:13:30-to-2020-06-11T18:18:30-switch.jsonl",
		},
	}

	for _, tt := range tests {
		start := tt.end.Add(time.Duration(tt.interval) * -time.Second)
		archivePath := GetPath(start, tt.end, tt.hostname)
		if archivePath != tt.expect {
			t.Errorf("Expected archive path '%v', but got: %v", tt.expect, archivePath)
		}
	}
}

func Test_WriteBadPath(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestWrite")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	archivePath := "/proc/bad/path/file.json"
	err = Write(archivePath, []byte("data"))
	if err == nil {
		t.Errorf("Expected an error but did not get one")
	}

}

func Test_Write(t *testing.T) {
	// Creates a tempdir for testing.
	dir, err := ioutil.TempDir("", "TestWriteUnwritableFile")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)

	jsonData := []byte{}
	for _, model := range testModels {
		jsonData = append(jsonData, MustMarshalJSON(model)...)
		jsonData = append(jsonData, '\n')
	}

	endTime := time.Now()
	startTime := endTime.Add(time.Duration(10) * -time.Second)
	archivePath := GetPath(startTime, endTime, "mlab2-abc0t.mlab-sandbox.measurement-lab.org")
	testArchivePath := fmt.Sprintf("%v/%v", dir, archivePath)

	err = Write(testArchivePath, jsonData)

	contents, err := ioutil.ReadFile(testArchivePath)
	rtx.Must(err, "Could not read test archive file")

	readModels := []Model{}
	data := bytes.NewReader(contents)
	dec := json.NewDecoder(data)
	for dec.More() {
		model := Model{}
		err := dec.Decode(&model)
		rtx.Must(err, "Failed to Decode JSON")
		readModels = append(readModels, model)
	}

	if !reflect.DeepEqual(testModels, readModels) {
		t.Errorf("Expected testModels:\n%v\nGot readModels:\n%v", testModels, readModels)
	}
}
