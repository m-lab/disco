package config

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/m-lab/go/rtx"
)

var goodYaml = `
- name: ifHCOutUcastPkts
  description: Test
  oidStub: .1.3.6.1.2.1.31.1.1.1.11
  mlabUplinkName: switch.unicast.uplink.tx
  mlabMachineName: switch.unicast.local.tx
`

var goodYamlStruct = Metric{
	Name:            "ifHCOutUcastPkts",
	Description:     "Test",
	OidStub:         ".1.3.6.1.2.1.31.1.1.1.11",
	MlabUplinkName:  "switch.unicast.uplink.tx",
	MlabMachineName: "switch.unicast.local.tx",
}

var badYaml = `
- badName: ifHCOutUcastPkts
  description: Egress unicast packets.
  mlabUplinkName: switch.unicast.uplink.tx
  mlabMachineName:
  -
stray data
`

func TestMissingYamlFile(t *testing.T) {
	_, err := New("/does/not/exist.yaml")
	if err == nil {
		t.Error("A non-existent YAML file should cause an error.")
	}
}

func TestBadYamlFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestBadYamlFile")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)
	rtx.Must(ioutil.WriteFile(dir+"/metrics.yaml", []byte(badYaml), 0644), "Could not write YAML to tempfile")

	_, err = New(dir + "/metrics.yaml")
	if err == nil {
		t.Error("A bad/corrupt YAML file should cause an error.")
	}
}

func TestGoodYamlFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestGoodYamlFile")
	rtx.Must(err, "Could not create tempdir")
	defer os.RemoveAll(dir)
	rtx.Must(ioutil.WriteFile(dir+"/metrics.yaml", []byte(goodYaml), 0644), "Could not write YAML to tempfile")

	c, err := New(dir + "/metrics.yaml")
	if len(c.Metrics) != 1 {
		t.Errorf("Expected 1 metric but got: %v", len(c.Metrics))
		return
	}

	m := c.Metrics[0]
	if !reflect.DeepEqual(goodYamlStruct, m) {
		t.Errorf("Expected Metric '%v' but got: %v", goodYamlStruct, m)
	}
}
