package archive

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/m-lab/go/rtx"
)

var testModel = Model{
	Experiment: "s1-abc0t.measurement-lab.org",
	Hostname:   "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
	Metric:     "switch.unicast.uplink.tx",
	Samples: []Sample{
		Sample{
			Timestamp: 1591845348,
			Value:     158,
		},
		Sample{
			Timestamp: 1591845358,
			Value:     132,
		},
	},
}

var expectedJSON = `{
    "experiment": "s1-abc0t.measurement-lab.org",
    "hostname": "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
    "metric": "switch.unicast.uplink.tx",
    "sample": [
        {
            "timestamp": 1591845348,
            "value": 158
        },
        {
            "timestamp": 1591845358,
            "value": 132
        }
    ]
}`

func Test_MustMarshalJSON(t *testing.T) {
	jsonData := MustMarshalJSON(testModel)

	if string(jsonData) != expectedJSON {
		t.Errorf("The collected JSON data does not match what was expected. Got: %v. Expected: %v", string(jsonData), expectedJSON)
	}
}

func Test_GetPath(t *testing.T) {
	tests := []struct {
		t        time.Time
		interval uint64
		hostname string
		expect   string
	}{
		{
			t:        time.Date(2010, 04, 18, 20, 34, 50, 0, time.UTC),
			interval: 60,
			hostname: "mlab2-abc0t.mlab-sandbox.measurement-lab.org",
			expect:   "2010/04/18/mlab2-abc0t.mlab-sandbox.measurement-lab.org/2010-04-18T20:33:50-to-2010-04-18T20:34:50-switch.json",
		},
		{
			t:        time.Date(1972, 07, 03, 11, 14, 10, 0, time.UTC),
			interval: 600,
			hostname: "mlab4-xyz03.mlab-staging.measurement-lab.org",
			expect:   "1972/07/03/mlab4-xyz03.mlab-staging.measurement-lab.org/1972-07-03T11:04:10-to-1972-07-03T11:14:10-switch.json",
		},
		{
			t:        time.Date(2020, 06, 11, 18, 18, 30, 0, time.UTC),
			interval: 300,
			hostname: "mlab1-qrs0t.mlab-sandbox.measurement-lab.org",
			expect:   "2020/06/11/mlab1-qrs0t.mlab-sandbox.measurement-lab.org/2020-06-11T18:13:30-to-2020-06-11T18:18:30-switch.json",
		},
	}

	for _, tt := range tests {
		archivePath := GetPath(tt.t, tt.hostname, tt.interval)
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

	jsonData := MustMarshalJSON(testModel)

	archivePath := GetPath(time.Now(), "mlab2-abc0t.mlab-sandbox.measurement-lab.org", 300)
	testArchivePath := fmt.Sprintf("%v/%v", dir, archivePath)

	err = Write(testArchivePath, jsonData)

	contents, err := ioutil.ReadFile(testArchivePath)
	rtx.Must(err, "Could not read test archive file")

	if string(contents) != expectedJSON {
		t.Error("Contents of written archive file do match expected contents")
	}

}
