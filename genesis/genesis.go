package genesis

import (
	"encoding/json"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/theQRL/go-qrl/core"
	"github.com/theQRL/go-qrl/generated"
)

type Genesis struct {
	core.Block
}

func (g *Genesis) GenesisBalance() []*generated.GenesisBalance {
	return g.PBData().GenesisBalance
}

func CreateGenesisBlock() (*Genesis, error) {
	yamlData, err := ioutil.ReadFile("genesis.yml")

	m := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(yamlData), &m)
	jsonData, err := json.Marshal(m)

	if err != nil {
		return nil, err
	}
	b := &Genesis{}

	b.Block.FromJSON(string(jsonData))

	return b, nil
}
