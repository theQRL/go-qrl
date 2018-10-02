package genesis

import (
	"github.com/cyyber/go-qrl/core"
	"github.com/cyyber/go-qrl/generated"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"encoding/json"
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
