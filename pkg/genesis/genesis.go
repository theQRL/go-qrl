package genesis

import (
	"github.com/ghodss/yaml"
	"github.com/theQRL/go-qrl/pkg/core/block"
	"github.com/theQRL/go-qrl/pkg/log"
	"io/ioutil"
)

func CreateGenesisBlock() (*block.Block, error) {
	yamlData, err := ioutil.ReadFile("../../pkg/genesis/genesis.yml") // TODO: Need better fix
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
