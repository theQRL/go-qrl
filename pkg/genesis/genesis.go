package genesis

import (
	"github.com/ghodss/yaml"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/log"
	"io/ioutil"
	"path"
	"runtime"
)

func CreateGenesisBlock() (*block.Block, error) {
	_, filename, _, _ := runtime.Caller(0)
	directory := path.Dir(filename) // Current Package path
	yamlData, err := ioutil.ReadFile(path.Join(directory, "genesis.yml"))
	l := log.GetLogger()

	if err != nil {
		l.Warn("Error while parsing Genesis.yml")
		l.Info(err.Error())
		return nil, err
	}

	jsonData, err := yaml.YAMLToJSON(yamlData)
	if err != nil {
		l.Info("Error while parsing Genesis")
		l.Info(err.Error())
		return nil, err
	}

	b := &block.Block{}
	b.FromJSON(string(jsonData))

	return b, nil
}
