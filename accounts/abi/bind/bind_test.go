// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package bind

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/theQRL/go-qrl/common"
)

var bindTests = []struct {
	name     string
	contract string
	bytecode []string
	abi      []string
	imports  string
	tester   string
	fsigs    []map[string]string
	libs     map[string]string
	aliases  map[string]string
	types    []string
}{

	// Test that the binding is available in combined and separate forms too
	{
		`Empty`,
		`contract NilContract {}`,
		[]string{`6080604052348015600e575f80fd5b50605e80601a5f395ff3fe60806040525f80fdfea2646970667358221220a80e8222d8e6b3f4dd49decd54be722a2bcde4c17e8140473413d4e0a8777b6e64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[]`},
		`
			"github.com/theQRL/go-qrl/common"
		`,
		`
			if b, err := NewEmpty(common.Address{}, nil); b == nil || err != nil {
				t.Fatalf("combined binding (%v) nil or error (%v) not nil", b, nil)
			}
			if b, err := NewEmptyCaller(common.Address{}, nil); b == nil || err != nil {
				t.Fatalf("caller binding (%v) nil or error (%v) not nil", b, nil)
			}
			if b, err := NewEmptyTransactor(common.Address{}, nil); b == nil || err != nil {
				t.Fatalf("transactor binding (%v) nil or error (%v) not nil", b, nil)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test that named and anonymous inputs are handled correctly
	{
		`InputChecker`, ``, []string{``},
		[]string{`[{"type":"function","name":"noInput","stateMutability":"view","inputs":[],"outputs":[]},{"type":"function","name":"namedInput","stateMutability":"view","inputs":[{"name":"str","type":"string"}],"outputs":[]},{"type":"function","name":"anonInput","stateMutability":"view","inputs":[{"name":"","type":"string"}],"outputs":[]},{"type":"function","name":"namedInputs","stateMutability":"view","inputs":[{"name":"str1","type":"string"},{"name":"str2","type":"string"}],"outputs":[]},{"type":"function","name":"anonInputs","stateMutability":"view","inputs":[{"name":"","type":"string"},{"name":"","type":"string"}],"outputs":[]},{"type":"function","name":"mixedInputs","stateMutability":"view","inputs":[{"name":"","type":"string"},{"name":"str","type":"string"}],"outputs":[]}]`},
		`
			"fmt"

			"github.com/theQRL/go-qrl/common"
		`,
		`
			if b, err := NewInputChecker(common.Address{}, nil); b == nil || err != nil {
				t.Fatalf("binding (%v) nil or error (%v) not nil", b, nil)
			} else if false { // Don't run, just compile and test types
				var err error

				err = b.NoInput(nil)
				err = b.NamedInput(nil, "")
				err = b.AnonInput(nil, "")
				err = b.NamedInputs(nil, "", "")
				err = b.AnonInputs(nil, "", "")
				err = b.MixedInputs(nil, "", "")

				fmt.Println(err)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test that named and anonymous outputs are handled correctly
	{
		`OutputChecker`, ``, []string{``},
		[]string{`[{"type":"function","name":"noOutput","stateMutability":"view","inputs":[],"outputs":[]},{"type":"function","name":"namedOutput","stateMutability":"view","inputs":[],"outputs":[{"name":"str","type":"string"}]},{"type":"function","name":"anonOutput","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"string"}]},{"type":"function","name":"namedOutputs","stateMutability":"view","inputs":[],"outputs":[{"name":"str1","type":"string"},{"name":"str2","type":"string"}]},{"type":"function","name":"collidingOutputs","stateMutability":"view","inputs":[],"outputs":[{"name":"str","type":"string"},{"name":"Str","type":"string"}]},{"type":"function","name":"anonOutputs","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"string"},{"name":"","type":"string"}]},{"type":"function","name":"mixedOutputs","stateMutability":"view","inputs":[],"outputs":[{"name":"","type":"string"},{"name":"str","type":"string"}]}]`},
		`
			"fmt"

			"github.com/theQRL/go-qrl/common"
		`,
		`
			if b, err := NewOutputChecker(common.Address{}, nil); b == nil || err != nil {
				t.Fatalf("binding (%v) nil or error (%v) not nil", b, nil)
			} else if false { // Don't run, just compile and test types
				var str1, str2 string
				var err error

				err              = b.NoOutput(nil)
				str1, err        = b.NamedOutput(nil)
				str1, err        = b.AnonOutput(nil)
				res, _          := b.NamedOutputs(nil)
				str1, str2, err  = b.CollidingOutputs(nil)
				str1, str2, err  = b.AnonOutputs(nil)
				str1, str2, err  = b.MixedOutputs(nil)

				fmt.Println(str1, str2, res.Str1, res.Str2, err)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that named, anonymous and indexed events are handled correctly
	{
		`EventChecker`, ``, []string{``},
		[]string{`[{"type":"event","name":"empty","inputs":[]},{"type":"event","name":"indexed","inputs":[{"name":"addr","type":"address","indexed":true},{"name":"num","type":"int256","indexed":true}]},{"type":"event","name":"mixed","inputs":[{"name":"addr","type":"address","indexed":true},{"name":"num","type":"int256"}]},{"type":"event","name":"anonymous","anonymous":true,"inputs":[]},{"type":"event","name":"dynamic","inputs":[{"name":"idxStr","type":"string","indexed":true},{"name":"idxDat","type":"bytes","indexed":true},{"name":"str","type":"string"},{"name":"dat","type":"bytes"}]},{"type":"event","name":"unnamed","inputs":[{"name":"","type":"uint256","indexed": true},{"name":"","type":"uint256","indexed":true}]}]`},
		`
			"fmt"
			"math/big"
			"reflect"

			"github.com/theQRL/go-qrl/common"
		`,
		`
			if e, err := NewEventChecker(common.Address{}, nil); e == nil || err != nil {
				t.Fatalf("binding (%v) nil or error (%v) not nil", e, nil)
			} else if false { // Don't run, just compile and test types
				var (
					err  error
				res  bool
					str  string
					dat  []byte
					hash common.Hash
				)
				_, err = e.FilterEmpty(nil)
				_, err = e.FilterIndexed(nil, []common.Address{}, []*big.Int{})

				mit, err := e.FilterMixed(nil, []common.Address{})

				res = mit.Next()  // Make sure the iterator has a Next method
				err = mit.Error() // Make sure the iterator has an Error method
				err = mit.Close() // Make sure the iterator has a Close method

				fmt.Println(mit.Event.Raw.BlockHash) // Make sure the raw log is contained within the results
				fmt.Println(mit.Event.Num)           // Make sure the unpacked non-indexed fields are present
				fmt.Println(mit.Event.Addr)          // Make sure the reconstructed indexed fields are present

				dit, err := e.FilterDynamic(nil, []string{}, [][]byte{})

				str  = dit.Event.Str    // Make sure non-indexed strings retain their type
				dat  = dit.Event.Dat    // Make sure non-indexed bytes retain their type
				hash = dit.Event.IdxStr // Make sure indexed strings turn into hashes
				hash = dit.Event.IdxDat // Make sure indexed bytes turn into hashes

				sink := make(chan *EventCheckerMixed)
				sub, err := e.WatchMixed(nil, sink, []common.Address{})
				defer sub.Unsubscribe()

				event := <-sink
				fmt.Println(event.Raw.BlockHash) // Make sure the raw log is contained within the results
				fmt.Println(event.Num)           // Make sure the unpacked non-indexed fields are present
				fmt.Println(event.Addr)          // Make sure the reconstructed indexed fields are present

				fmt.Println(res, str, dat, hash, err)

				oit, err := e.FilterUnnamed(nil, []*big.Int{}, []*big.Int{})

				arg0  := oit.Event.Arg0    // Make sure unnamed arguments are handled correctly
				arg1  := oit.Event.Arg1    // Make sure unnamed arguments are handled correctly
				fmt.Println(arg0, arg1)
			}
			// Run a tiny reflection test to ensure disallowed methods don't appear
			if _, ok := reflect.TypeOf(&EventChecker{}).MethodByName("FilterAnonymous"); ok {
			t.Errorf("binding has disallowed method (FilterAnonymous)")
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test that contract interactions (deploy, transact and call) generate working code
	{
		`Interactor`,
		`
			contract Interactor {
				string public deployString;
				string public transactString;

				constructor(string memory str) public {
					deployString = str;
				}

				function transact(string memory str) public {
					transactString = str;
				}
			}
		`,
		[]string{`608060405234801562000010575f80fd5b5060405162000c6638038062000c668339818101604052810190620000369190620001d3565b805f908162000046919062000459565b50506200053d565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b620000af8262000067565b810181811067ffffffffffffffff82111715620000d157620000d062000077565b5b80604052505050565b5f620000e56200004e565b9050620000f38282620000a4565b919050565b5f67ffffffffffffffff82111562000115576200011462000077565b5b620001208262000067565b9050602081019050919050565b5f5b838110156200014c5780820151818401526020810190506200012f565b5f8484015250505050565b5f6200016d6200016784620000f8565b620000da565b9050828152602081018484840111156200018c576200018b62000063565b5b620001998482856200012d565b509392505050565b5f82601f830112620001b857620001b76200005f565b5b8151620001ca84826020860162000157565b91505092915050565b5f60208284031215620001eb57620001ea62000057565b5b5f82015167ffffffffffffffff8111156200020b576200020a6200005b565b5b6200021984828501620001a1565b91505092915050565b5f81519050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52602260045260245ffd5b5f60028204905060018216806200027157607f821691505b6020821081036200028757620002866200022c565b5b50919050565b5f819050815f5260205f209050919050565b5f6020601f8301049050919050565b5f82821b905092915050565b5f60088302620002eb7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82620002ae565b620002f78683620002ae565b95508019841693508086168417925050509392505050565b5f819050919050565b5f819050919050565b5f620003416200033b62000335846200030f565b62000318565b6200030f565b9050919050565b5f819050919050565b6200035c8362000321565b620003746200036b8262000348565b848454620002ba565b825550505050565b5f90565b6200038a6200037c565b6200039781848462000351565b505050565b5b81811015620003be57620003b25f8262000380565b6001810190506200039d565b5050565b601f8211156200040d57620003d7816200028d565b620003e2846200029f565b81016020851015620003f2578190505b6200040a62000401856200029f565b8301826200039c565b50505b505050565b5f82821c905092915050565b5f6200042f5f198460080262000412565b1980831691505092915050565b5f6200044983836200041e565b9150826002028217905092915050565b620004648262000222565b67ffffffffffffffff81111562000480576200047f62000077565b5b6200048c825462000259565b62000499828285620003c2565b5f60209050601f831160018114620004cf575f8415620004ba578287015190505b620004c685826200043c565b86555062000535565b601f198416620004df866200028d565b5f5b828110156200050857848901518255600182019150602085019450602081019050620004e1565b8683101562000528578489015162000524601f8916826200041e565b8355505b6001600288020188555050505b505050505050565b61071b806200054b5f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c80630d86a0e1146100435780636874e80914610061578063d736c5131461007f575b5f80fd5b61004b61009b565b604051610058919061024f565b60405180910390f35b610069610127565b604051610076919061024f565b60405180910390f35b610099600480360381019061009491906103ac565b6101b2565b005b600180546100a890610420565b80601f01602080910402602001604051908101604052809291908181526020018280546100d490610420565b801561011f5780601f106100f65761010080835404028352916020019161011f565b820191905f5260205f20905b81548152906001019060200180831161010257829003601f168201915b505050505081565b5f805461013390610420565b80601f016020809104026020016040519081016040528092919081815260200182805461015f90610420565b80156101aa5780601f10610181576101008083540402835291602001916101aa565b820191905f5260205f20905b81548152906001019060200180831161018d57829003601f168201915b505050505081565b80600190816101c191906105f6565b5050565b5f81519050919050565b5f82825260208201905092915050565b5f5b838110156101fc5780820151818401526020810190506101e1565b5f8484015250505050565b5f601f19601f8301169050919050565b5f610221826101c5565b61022b81856101cf565b935061023b8185602086016101df565b61024481610207565b840191505092915050565b5f6020820190508181035f8301526102678184610217565b905092915050565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f80fd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6102be82610207565b810181811067ffffffffffffffff821117156102dd576102dc610288565b5b80604052505050565b5f6102ef61026f565b90506102fb82826102b5565b919050565b5f67ffffffffffffffff82111561031a57610319610288565b5b61032382610207565b9050602081019050919050565b828183375f83830152505050565b5f61035061034b84610300565b6102e6565b90508281526020810184848401111561036c5761036b610284565b5b610377848285610330565b509392505050565b5f82601f83011261039357610392610280565b5b81356103a384826020860161033e565b91505092915050565b5f602082840312156103c1576103c0610278565b5b5f82013567ffffffffffffffff8111156103de576103dd61027c565b5b6103ea8482850161037f565b91505092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52602260045260245ffd5b5f600282049050600182168061043757607f821691505b60208210810361044a576104496103f3565b5b50919050565b5f819050815f5260205f209050919050565b5f6020601f8301049050919050565b5f82821b905092915050565b5f600883026104ac7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82610471565b6104b68683610471565b95508019841693508086168417925050509392505050565b5f819050919050565b5f819050919050565b5f6104fa6104f56104f0846104ce565b6104d7565b6104ce565b9050919050565b5f819050919050565b610513836104e0565b61052761051f82610501565b84845461047d565b825550505050565b5f90565b61053b61052f565b61054681848461050a565b505050565b5b818110156105695761055e5f82610533565b60018101905061054c565b5050565b601f8211156105ae5761057f81610450565b61058884610462565b81016020851015610597578190505b6105ab6105a385610462565b83018261054b565b50505b505050565b5f82821c905092915050565b5f6105ce5f19846008026105b3565b1980831691505092915050565b5f6105e683836105bf565b9150826002028217905092915050565b6105ff826101c5565b67ffffffffffffffff81111561061857610617610288565b5b6106228254610420565b61062d82828561056d565b5f60209050601f83116001811461065e575f841561064c578287015190505b61065685826105db565b8655506106bd565b601f19841661066c86610450565b5f5b828110156106935784890151825560018201915060208501945060208101905061066e565b868310156106b057848901516106ac601f8916826105bf565b8355505b6001600288020188555050505b50505050505056fea264697066735822122048f755c8975827629de08a59d504ddc84809f226a4ff613ccd30b16de7178f7464687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[{"internalType":"string","name":"str","type":"string"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[],"name":"deployString","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string","name":"str","type":"string"}],"name":"transact","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"transactString","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy an interaction tester contract and call a transaction on it
			_, _, interactor, err := DeployInteractor(auth, sim, "Deploy string")
			if err != nil {
				t.Fatalf("Failed to deploy interactor contract: %v", err)
			}
			if _, err := interactor.Transact(auth, "Transact string"); err != nil {
				t.Fatalf("Failed to transact with interactor contract: %v", err)
			}
			// Commit all pending transactions in the simulator and check the contract state
			sim.Commit()

			if str, err := interactor.DeployString(nil); err != nil {
				t.Fatalf("Failed to retrieve deploy string: %v", err)
			} else if str != "Deploy string" {
				t.Fatalf("Deploy string mismatch: have '%s', want 'Deploy string'", str)
			}
			if str, err := interactor.TransactString(nil); err != nil {
				t.Fatalf("Failed to retrieve transact string: %v", err)
			} else if str != "Transact string" {
				t.Fatalf("Transact string mismatch: have '%s', want 'Transact string'", str)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that plain values can be properly returned and deserialized
	{
		`Getter`,
		`
			contract Getter {
				function getter() public pure returns (string memory, int, bytes32) {
					return ("Hi", 1, keccak256(""));
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506102038061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c8063993a04b71461002d575b5f80fd5b61003561004d565b60405161004493929190610171565b60405180910390f35b60605f8060017fc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a4706040518060400160405280600281526020017f48690000000000000000000000000000000000000000000000000000000000008152509190925092509250909192565b5f81519050919050565b5f82825260208201905092915050565b5f5b838110156100ee5780820151818401526020810190506100d3565b5f8484015250505050565b5f601f19601f8301169050919050565b5f610113826100b7565b61011d81856100c1565b935061012d8185602086016100d1565b610136816100f9565b840191505092915050565b5f819050919050565b61015381610141565b82525050565b5f819050919050565b61016b81610159565b82525050565b5f6060820190508181035f8301526101898186610109565b9050610198602083018561014a565b6101a56040830184610162565b94935050505056fea26469706673582212207a230398b0ef392b8b824a86ad91227cc87cd336309d16ffbda0a538af892fde64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"getter","outputs":[{"internalType":"string","name":"","type":"string"},{"internalType":"int256","name":"","type":"int256"},{"internalType":"bytes32","name":"","type":"bytes32"}],"stateMutability":"pure","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a tuple tester contract and execute a structured call on it
			_, _, getter, err := DeployGetter(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy getter contract: %v", err)
			}
			sim.Commit()

			if str, num, _, err := getter.Getter(nil); err != nil {
				t.Fatalf("Failed to call anonymous field retriever: %v", err)
			} else if str != "Hi" || num.Cmp(big.NewInt(1)) != 0 {
				t.Fatalf("Retrieved value mismatch: have %v/%v, want %v/%v", str, num, "Hi", 1)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that tuples can be properly returned and deserialized
	{
		`Tupler`,
		`
			contract Tupler {
				function tuple() public pure returns (string memory a, int b, bytes32 c) {
					return ("Hi", 1, keccak256(""));
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506102038061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c80633175aae21461002d575b5f80fd5b61003561004d565b60405161004493929190610171565b60405180910390f35b60605f8060017fc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a4706040518060400160405280600281526020017f48690000000000000000000000000000000000000000000000000000000000008152509190925092509250909192565b5f81519050919050565b5f82825260208201905092915050565b5f5b838110156100ee5780820151818401526020810190506100d3565b5f8484015250505050565b5f601f19601f8301169050919050565b5f610113826100b7565b61011d81856100c1565b935061012d8185602086016100d1565b610136816100f9565b840191505092915050565b5f819050919050565b61015381610141565b82525050565b5f819050919050565b61016b81610159565b82525050565b5f6060820190508181035f8301526101898186610109565b9050610198602083018561014a565b6101a56040830184610162565b94935050505056fea2646970667358221220b9820e35b3522ce3bcc5e68cb8eaa61259ee94b328a87b4835d78ee204326f5e64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"tuple","outputs":[{"internalType":"string","name":"a","type":"string"},{"internalType":"int256","name":"b","type":"int256"},{"internalType":"bytes32","name":"c","type":"bytes32"}],"stateMutability":"pure","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a tuple tester contract and execute a structured call on it
			_, _, tupler, err := DeployTupler(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy tupler contract: %v", err)
			}
			sim.Commit()

			if res, err := tupler.Tuple(nil); err != nil {
				t.Fatalf("Failed to call structure retriever: %v", err)
			} else if res.A != "Hi" || res.B.Cmp(big.NewInt(1)) != 0 {
				t.Fatalf("Retrieved value mismatch: have %v/%v, want %v/%v", res.A, res.B, "Hi", 1)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},

	// Tests that arrays/slices can be properly returned and deserialized.
	// Only addresses are tested, remainder just compiled to keep the test small.
	{
		`Slicer`,
		`
			contract Slicer {
				function echoAddresses(address[] memory input) public pure returns (address[] memory output) {
					return input;
				}
				function echoInts(int[] memory input) public pure returns (int[] memory output) {
					return input;
				}
				function echoFancyInts(uint24[23] memory input) public pure returns (uint24[23] memory output) {
					return input;
				}
				function echoBools(bool[] memory input) public pure returns (bool[] memory output) {
					return input;
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b50610a838061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061004a575f3560e01c8063be1127a31461004e578063d88becc01461007e578063e15a3db7146100ae578063f637e589146100de575b5f80fd5b6100686004803603810190610063919061031a565b61010e565b6040516100759190610418565b60405180910390f35b6100986004803603810190610093919061051e565b610118565b6040516100a591906105ef565b60405180910390f35b6100c860048036038101906100c391906106fc565b610128565b6040516100d591906107fa565b60405180910390f35b6100f860048036038101906100f3919061090f565b610132565b6040516101059190610a0d565b60405180910390f35b6060819050919050565b61012061013c565b819050919050565b6060819050919050565b6060819050919050565b604051806102e00160405280601790602082028036833780820191505090505090565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6101ba82610174565b810181811067ffffffffffffffff821117156101d9576101d8610184565b5b80604052505050565b5f6101eb61015f565b90506101f782826101b1565b919050565b5f67ffffffffffffffff82111561021657610215610184565b5b602082029050602081019050919050565b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6102548261022b565b9050919050565b6102648161024a565b811461026e575f80fd5b50565b5f8135905061027f8161025b565b92915050565b5f610297610292846101fc565b6101e2565b905080838252602082019050602084028301858111156102ba576102b9610227565b5b835b818110156102e357806102cf8882610271565b8452602084019350506020810190506102bc565b5050509392505050565b5f82601f83011261030157610300610170565b5b8135610311848260208601610285565b91505092915050565b5f6020828403121561032f5761032e610168565b5b5f82013567ffffffffffffffff81111561034c5761034b61016c565b5b610358848285016102ed565b91505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b6103938161024a565b82525050565b5f6103a4838361038a565b60208301905092915050565b5f602082019050919050565b5f6103c682610361565b6103d0818561036b565b93506103db8361037b565b805f5b8381101561040b5781516103f28882610399565b97506103fd836103b0565b9250506001810190506103de565b5085935050505092915050565b5f6020820190508181035f83015261043081846103bc565b905092915050565b5f67ffffffffffffffff82111561045257610451610184565b5b602082029050919050565b5f62ffffff82169050919050565b6104748161045d565b811461047e575f80fd5b50565b5f8135905061048f8161046b565b92915050565b5f6104a76104a284610438565b6101e2565b905080602084028301858111156104c1576104c0610227565b5b835b818110156104ea57806104d68882610481565b8452602084019350506020810190506104c3565b5050509392505050565b5f82601f83011261050857610507610170565b5b6017610515848285610495565b91505092915050565b5f6102e0828403121561053457610533610168565b5b5f610541848285016104f4565b91505092915050565b5f60179050919050565b5f81905092915050565b5f819050919050565b6105708161045d565b82525050565b5f6105818383610567565b60208301905092915050565b5f602082019050919050565b6105a28161054a565b6105ac8184610554565b92506105b78261055e565b805f5b838110156105e75781516105ce8782610576565b96506105d98361058d565b9250506001810190506105ba565b505050505050565b5f6102e0820190506106035f830184610599565b92915050565b5f67ffffffffffffffff82111561062357610622610184565b5b602082029050602081019050919050565b5f819050919050565b61064681610634565b8114610650575f80fd5b50565b5f813590506106618161063d565b92915050565b5f61067961067484610609565b6101e2565b9050808382526020820190506020840283018581111561069c5761069b610227565b5b835b818110156106c557806106b18882610653565b84526020840193505060208101905061069e565b5050509392505050565b5f82601f8301126106e3576106e2610170565b5b81356106f3848260208601610667565b91505092915050565b5f6020828403121561071157610710610168565b5b5f82013567ffffffffffffffff81111561072e5761072d61016c565b5b61073a848285016106cf565b91505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b61077581610634565b82525050565b5f610786838361076c565b60208301905092915050565b5f602082019050919050565b5f6107a882610743565b6107b2818561074d565b93506107bd8361075d565b805f5b838110156107ed5781516107d4888261077b565b97506107df83610792565b9250506001810190506107c0565b5085935050505092915050565b5f6020820190508181035f830152610812818461079e565b905092915050565b5f67ffffffffffffffff82111561083457610833610184565b5b602082029050602081019050919050565b5f8115159050919050565b61085981610845565b8114610863575f80fd5b50565b5f8135905061087481610850565b92915050565b5f61088c6108878461081a565b6101e2565b905080838252602082019050602084028301858111156108af576108ae610227565b5b835b818110156108d857806108c48882610866565b8452602084019350506020810190506108b1565b5050509392505050565b5f82601f8301126108f6576108f5610170565b5b813561090684826020860161087a565b91505092915050565b5f6020828403121561092457610923610168565b5b5f82013567ffffffffffffffff8111156109415761094061016c565b5b61094d848285016108e2565b91505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b61098881610845565b82525050565b5f610999838361097f565b60208301905092915050565b5f602082019050919050565b5f6109bb82610956565b6109c58185610960565b93506109d083610970565b805f5b83811015610a005781516109e7888261098e565b97506109f2836109a5565b9250506001810190506109d3565b5085935050505092915050565b5f6020820190508181035f830152610a2581846109b1565b90509291505056fea264697066735822122062f72cf1c8bb65d50e3eb754ae8b0745f9c78b7f42e37fb3cab7a028cccbb10964687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[{"internalType":"address[]","name":"input","type":"address[]"}],"name":"echoAddresses","outputs":[{"internalType":"address[]","name":"output","type":"address[]"}],"stateMutability":"pure","type":"function"},{"inputs":[{"internalType":"bool[]","name":"input","type":"bool[]"}],"name":"echoBools","outputs":[{"internalType":"bool[]","name":"output","type":"bool[]"}],"stateMutability":"pure","type":"function"},{"inputs":[{"internalType":"uint24[23]","name":"input","type":"uint24[23]"}],"name":"echoFancyInts","outputs":[{"internalType":"uint24[23]","name":"output","type":"uint24[23]"}],"stateMutability":"pure","type":"function"},{"inputs":[{"internalType":"int256[]","name":"input","type":"int256[]"}],"name":"echoInts","outputs":[{"internalType":"int256[]","name":"output","type":"int256[]"}],"stateMutability":"pure","type":"function"}]`},
		`
			"math/big"
			"reflect"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/common"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a slice tester contract and execute a n array call on it
			_, _, slicer, err := DeploySlicer(auth, sim)
			if err != nil {
					t.Fatalf("Failed to deploy slicer contract: %v", err)
			}
			sim.Commit()

			if out, err := slicer.EchoAddresses(nil, []common.Address{auth.From, common.Address{}}); err != nil {
					t.Fatalf("Failed to call slice echoer: %v", err)
			} else if !reflect.DeepEqual(out, []common.Address{auth.From, common.Address{}}) {
					t.Fatalf("Slice return mismatch: have %v, want %v", out, []common.Address{auth.From, common.Address{}})
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that structs are correctly unpacked
	{

		`Structs`,
		`
			pragma experimental ABIEncoderV2;
			contract Structs {
				struct A {
					bytes32 B;
				}

				function F() public view returns (A[] memory a, uint256[] memory c, bool[] memory d) {
					A[] memory a = new A[](2);
					a[0].B = bytes32(uint256(1234) << 96);
					uint256[] memory c;
					bool[] memory d;
					return (a, c, d);
				}

				function G() public view returns (A[] memory a) {
					A[] memory a = new A[](2);
					a[0].B = bytes32(uint256(1234) << 96);
					return a;
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506105298061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c806328811f59146100385780636fecb62314610058575b5f80fd5b610040610076565b60405161004f9392919061040f565b60405180910390f35b610060610112565b60405161006d9190610459565b60405180910390f35b60608060605f600267ffffffffffffffff81111561009757610096610479565b5b6040519080825280602002602001820160405280156100d057816020015b6100bd61019e565b8152602001906001900390816100b55790505b50905060606104d2901b5f1b815f815181106100ef576100ee6104a6565b5b60200260200101515f018181525050606080828282955095509550505050909192565b60605f600267ffffffffffffffff8111156101305761012f610479565b5b60405190808252806020026020018201604052801561016957816020015b61015661019e565b81526020019060019003908161014e5790505b50905060606104d2901b5f1b815f81518110610188576101876104a6565b5b60200260200101515f0181815250508091505090565b60405180602001604052805f80191681525090565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b5f819050919050565b6101ee816101dc565b82525050565b602082015f8201516102085f8501826101e5565b50505050565b5f61021983836101f4565b60208301905092915050565b5f602082019050919050565b5f61023b826101b3565b61024581856101bd565b9350610250836101cd565b805f5b83811015610280578151610267888261020e565b975061027283610225565b925050600181019050610253565b5085935050505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b5f819050919050565b6102c8816102b6565b82525050565b5f6102d983836102bf565b60208301905092915050565b5f602082019050919050565b5f6102fb8261028d565b6103058185610297565b9350610310836102a7565b805f5b8381101561034057815161032788826102ce565b9750610332836102e5565b925050600181019050610313565b5085935050505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b5f8115159050919050565b61038a81610376565b82525050565b5f61039b8383610381565b60208301905092915050565b5f602082019050919050565b5f6103bd8261034d565b6103c78185610357565b93506103d283610367565b805f5b838110156104025781516103e98882610390565b97506103f4836103a7565b9250506001810190506103d5565b5085935050505092915050565b5f6060820190508181035f8301526104278186610231565b9050818103602083015261043b81856102f1565b9050818103604083015261044f81846103b3565b9050949350505050565b5f6020820190508181035f8301526104718184610231565b905092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52603260045260245ffdfea26469706673582212209ff141d6f80fdf2d5ef31c8e26959d62a2f18bf631560e7414f77cf0e21dfa6164687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"F","outputs":[{"components":[{"internalType":"bytes32","name":"B","type":"bytes32"}],"internalType":"struct Structs.A[]","name":"a","type":"tuple[]"},{"internalType":"uint256[]","name":"c","type":"uint256[]"},{"internalType":"bool[]","name":"d","type":"bool[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"G","outputs":[{"components":[{"internalType":"bytes32","name":"B","type":"bytes32"}],"internalType":"struct Structs.A[]","name":"a","type":"tuple[]"}],"stateMutability":"view","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a structs method invoker contract and execute its default method
			_, _, structs, err := DeployStructs(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy defaulter contract: %v", err)
			}
			sim.Commit()
			opts := bind.CallOpts{}
			if _, err := structs.F(&opts); err != nil {
				t.Fatalf("Failed to invoke F method: %v", err)
			}
			if _, err := structs.G(&opts); err != nil {
				t.Fatalf("Failed to invoke G method: %v", err)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that non-existent contracts are reported as such (though only simulator test)
	{
		`NonExistent`,
		`
			contract NonExistent {
				function String() public pure returns(string memory) {
					return "I don't exist";
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506101888061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c8063f97a60051461002d575b5f80fd5b61003561004b565b6040516100429190610112565b60405180910390f35b60606040518060400160405280600d81526020017f4920646f6e277420657869737400000000000000000000000000000000000000815250905090565b5f81519050919050565b5f82825260208201905092915050565b5f5b838110156100bf5780820151818401526020810190506100a4565b5f8484015250505050565b5f601f19601f8301169050919050565b5f6100e482610088565b6100ee8185610092565b93506100fe8185602086016100a2565b610107816100ca565b840191505092915050565b5f6020820190508181035f83015261012a81846100da565b90509291505056fea264697066735822122010c82b9beb9a9021a9b7fac44becbb220ef3e27d2ccb5d33c9d3fc268440d38964687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"String","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"pure","type":"function"}]`},
		`
			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/common"
			"github.com/theQRL/go-qrl/core"
		`,
		`
			// Create a simulator and wrap a non-deployed contract

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{}, uint64(10000000000))
			defer sim.Close()

			nonexistent, err := NewNonExistent(common.Address{}, sim)
			if err != nil {
				t.Fatalf("Failed to access non-existent contract: %v", err)
			}
			// Ensure that contract calls fail with the appropriate error
			if res, err := nonexistent.String(nil); err == nil {
				t.Fatalf("Call succeeded on non-existent contract: %v", res)
			} else if (err != bind.ErrNoCode) {
				t.Fatalf("Error mismatch: have %v, want %v", err, bind.ErrNoCode)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	{
		`NonExistentStruct`,
		`
			contract NonExistentStruct {
				function Struct() public pure returns(uint256 a, uint256 b) {
					return (10, 10);
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b5060e18061001c5f395ff3fe6080604052348015600e575f80fd5b50600436106026575f3560e01c8063d5f6622514602a575b5f80fd5b60306045565b604051603c9291906068565b60405180910390f35b5f80600a80915091509091565b5f819050919050565b6062816052565b82525050565b5f60408201905060795f830185605b565b60846020830184605b565b939250505056fea2646970667358221220f23812f4bf6452d78f05884b269e72452b4a21fe9c95ddcba91f86b8a106130f64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"Struct","outputs":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256","name":"b","type":"uint256"}],"stateMutability":"pure","type":"function"}]`},
		`
			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/common"
			"github.com/theQRL/go-qrl/core"
		`,
		`
			// Create a simulator and wrap a non-deployed contract

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{}, uint64(10000000000))
			defer sim.Close()

			nonexistent, err := NewNonExistentStruct(common.Address{}, sim)
			if err != nil {
				t.Fatalf("Failed to access non-existent contract: %v", err)
			}
			// Ensure that contract calls fail with the appropriate error
			if res, err := nonexistent.Struct(nil); err == nil {
				t.Fatalf("Call succeeded on non-existent contract: %v", res)
			} else if (err != bind.ErrNoCode) {
				t.Fatalf("Error mismatch: have %v, want %v", err, bind.ErrNoCode)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that gas estimation works for contracts with weird gas mechanics too.
	{
		`FunkyGasPattern`,
		`
			contract FunkyGasPattern {
				string public field;

				function SetField(string memory value) public {
					// This check will screw gas estimation! Good, good!
					if (gasleft() < 100000) {
						revert();
					}
					field = value;
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506106748061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c806323fcf32a146100385780634f28bf0e14610054575b5f80fd5b610052600480360381019061004d919061026b565b610072565b005b61005c610093565b604051610069919061032c565b60405180910390f35b620186a05a1015610081575f80fd5b805f908161008f919061054f565b5050565b5f805461009f90610379565b80601f01602080910402602001604051908101604052809291908181526020018280546100cb90610379565b80156101165780601f106100ed57610100808354040283529160200191610116565b820191905f5260205f20905b8154815290600101906020018083116100f957829003601f168201915b505050505081565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b61017d82610137565b810181811067ffffffffffffffff8211171561019c5761019b610147565b5b80604052505050565b5f6101ae61011e565b90506101ba8282610174565b919050565b5f67ffffffffffffffff8211156101d9576101d8610147565b5b6101e282610137565b9050602081019050919050565b828183375f83830152505050565b5f61020f61020a846101bf565b6101a5565b90508281526020810184848401111561022b5761022a610133565b5b6102368482856101ef565b509392505050565b5f82601f8301126102525761025161012f565b5b81356102628482602086016101fd565b91505092915050565b5f602082840312156102805761027f610127565b5b5f82013567ffffffffffffffff81111561029d5761029c61012b565b5b6102a98482850161023e565b91505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f5b838110156102e95780820151818401526020810190506102ce565b5f8484015250505050565b5f6102fe826102b2565b61030881856102bc565b93506103188185602086016102cc565b61032181610137565b840191505092915050565b5f6020820190508181035f83015261034481846102f4565b905092915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52602260045260245ffd5b5f600282049050600182168061039057607f821691505b6020821081036103a3576103a261034c565b5b50919050565b5f819050815f5260205f209050919050565b5f6020601f8301049050919050565b5f82821b905092915050565b5f600883026104057fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff826103ca565b61040f86836103ca565b95508019841693508086168417925050509392505050565b5f819050919050565b5f819050919050565b5f61045361044e61044984610427565b610430565b610427565b9050919050565b5f819050919050565b61046c83610439565b6104806104788261045a565b8484546103d6565b825550505050565b5f90565b610494610488565b61049f818484610463565b505050565b5b818110156104c2576104b75f8261048c565b6001810190506104a5565b5050565b601f821115610507576104d8816103a9565b6104e1846103bb565b810160208510156104f0578190505b6105046104fc856103bb565b8301826104a4565b50505b505050565b5f82821c905092915050565b5f6105275f198460080261050c565b1980831691505092915050565b5f61053f8383610518565b9150826002028217905092915050565b610558826102b2565b67ffffffffffffffff81111561057157610570610147565b5b61057b8254610379565b6105868282856104c6565b5f60209050601f8311600181146105b7575f84156105a5578287015190505b6105af8582610534565b865550610616565b601f1984166105c5866103a9565b5f5b828110156105ec578489015182556001820191506020850194506020810190506105c7565b868310156106095784890151610605601f891682610518565b8355505b6001600288020188555050505b50505050505056fea26469706673582212209bd6b70d8a827a2ce0a7efc1ccffeaa5cb8e022e5b7014b49c35110776cdf59864687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[{"internalType":"string","name":"value","type":"string"}],"name":"SetField","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"field","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a funky gas pattern contract
			_, _, limiter, err := DeployFunkyGasPattern(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy funky contract: %v", err)
			}
			sim.Commit()

			// Set the field with automatic estimation and check that it succeeds
			if _, err := limiter.SetField(auth, "automatic"); err != nil {
				t.Fatalf("Failed to call automatically gased transaction: %v", err)
			}
			sim.Commit()

			if field, _ := limiter.Field(nil); field != "automatic" {
				t.Fatalf("Field mismatch: have %v, want %v", field, "automatic")
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test that constant functions can be called from an (optional) specified address
	{
		`CallFrom`,
		`
			contract CallFrom {
				function callFrom() public view returns(address) {
					return msg.sender;
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b5060f38061001c5f395ff3fe6080604052348015600e575f80fd5b50600436106026575f3560e01c806349f8e98214602a575b5f80fd5b60306044565b604051603b91906086565b60405180910390f35b5f33905090565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f607282604b565b9050919050565b608081606a565b82525050565b5f60208201905060975f8301846079565b9291505056fea2646970667358221220d41a6d88f4848d3cb238749937c30ff28594b46691a35d6e5e412aac14e6511064687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"callFrom","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/common"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a sender tester contract and execute a structured call on it
			_, _, callfrom, err := DeployCallFrom(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy sender contract: %v", err)
			}
			sim.Commit()

			if res, err := callfrom.CallFrom(nil); err != nil {
				t.Errorf("Failed to call constant function: %v", err)
			} else if res != (common.Address{}) {
				t.Errorf("Invalid address returned, want: %x, got: %x", (common.Address{}), res)
			}

			for _, addr := range []common.Address{common.Address{}, common.Address{1}, common.Address{2}} {
				if res, err := callfrom.CallFrom(&bind.CallOpts{From: addr}); err != nil {
					t.Fatalf("Failed to call constant function: %v", err)
				} else if res != addr {
					t.Fatalf("Invalid address returned, want: %x, got: %x", addr, res)
				}
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that methods and returns with underscores inside work correctly.
	{
		`Underscorer`,
		`
			contract Underscorer {
				function UnderscoredOutput() public pure returns (int _int, string memory _string) {
					return (314, "pi");
				}
				function LowerLowerCollision() public pure returns (int _res, int res) {
					return (1, 2);
				}
				function LowerUpperCollision() public pure returns (int _res, int Res) {
					return (1, 2);
				}
				function UpperLowerCollision() public pure returns (int _Res, int res) {
					return (1, 2);
				}
				function UpperUpperCollision() public pure returns (int _Res, int Res) {
					return (1, 2);
				}
				function _under_scored_func() public pure returns (int _int) {
					return 0;
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506103038061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610060575f3560e01c806303a592131461006457806346546dbe1461008357806367e6633d146100a1578063af7486ab146100c0578063e02ab24d146100df578063e409ca45146100fe575b5f80fd5b61006c61011d565b60405161007a9291906101b5565b60405180910390f35b61008b61012b565b60405161009891906101dc565b60405180910390f35b6100a961012f565b6040516100b792919061027f565b60405180910390f35b6100c8610173565b6040516100d69291906101b5565b60405180910390f35b6100e7610181565b6040516100f59291906101b5565b60405180910390f35b61010661018f565b6040516101149291906101b5565b60405180910390f35b5f8060016002915091509091565b5f90565b5f606061013a6040518060400160405280600281526020017f7069000000000000000000000000000000000000000000000000000000000000815250915091509091565b5f8060016002915091509091565b5f8060016002915091509091565b5f8060016002915091509091565b5f819050919050565b6101af8161019d565b82525050565b5f6040820190506101c85f8301856101a6565b6101d560208301846101a6565b9392505050565b5f6020820190506101ef5f8301846101a6565b92915050565b5f81519050919050565b5f82825260208201905092915050565b5f5b8381101561022c578082015181840152602081019050610211565b5f8484015250505050565b5f601f19601f8301169050919050565b5f610251826101f5565b61025b81856101ff565b935061026b81856020860161020f565b61027481610237565b840191505092915050565b5f6040820190506102925f8301856101a6565b81810360208301526102a48184610247565b9050939250505056fea2646970667358221220129e1361940f55dc253048c532d73c971d7a9f3e31eeda8bc9d54770d918036564687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[],"name":"LowerLowerCollision","outputs":[{"internalType":"int256","name":"_res","type":"int256"},{"internalType":"int256","name":"res","type":"int256"}],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"LowerUpperCollision","outputs":[{"internalType":"int256","name":"_res","type":"int256"},{"internalType":"int256","name":"Res","type":"int256"}],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"UnderscoredOutput","outputs":[{"internalType":"int256","name":"_int","type":"int256"},{"internalType":"string","name":"_string","type":"string"}],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"UpperLowerCollision","outputs":[{"internalType":"int256","name":"_Res","type":"int256"},{"internalType":"int256","name":"res","type":"int256"}],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"UpperUpperCollision","outputs":[{"internalType":"int256","name":"_Res","type":"int256"},{"internalType":"int256","name":"Res","type":"int256"}],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"_under_scored_func","outputs":[{"internalType":"int256","name":"_int","type":"int256"}],"stateMutability":"pure","type":"function"}]`},
		`
			"fmt"
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a underscorer tester contract and execute a structured call on it
			_, _, underscorer, err := DeployUnderscorer(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy underscorer contract: %v", err)
			}
			sim.Commit()

			// Verify that underscored return values correctly parse into structs
			if res, err := underscorer.UnderscoredOutput(nil); err != nil {
				t.Errorf("Failed to call constant function: %v", err)
			} else if res.Int.Cmp(big.NewInt(314)) != 0 || res.String != "pi" {
				t.Errorf("Invalid result, want: {314, \"pi\"}, got: %+v", res)
			}
			// Verify that underscored and non-underscored name collisions force tuple outputs
			var a, b *big.Int

			a, b, _ = underscorer.LowerLowerCollision(nil)
			a, b, _ = underscorer.LowerUpperCollision(nil)
			a, b, _ = underscorer.UpperLowerCollision(nil)
			a, b, _ = underscorer.UpperUpperCollision(nil)
			a, _ = underscorer.UnderScoredFunc(nil)

			fmt.Println(a, b, err)
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Tests that logs can be successfully filtered and decoded.
	{
		`Eventer`,
		`
			contract Eventer {
				event SimpleEvent (
					address indexed Addr,
					bytes32 indexed Id,
					bool    indexed Flag,
					uint    Value
				);
				function raiseSimpleEvent(address addr, bytes32 id, bool flag, uint value) {
					SimpleEvent(addr, id, flag, value);
				}

				event NodataEvent (
					uint   indexed Number,
					int16  indexed Short,
					uint32 indexed Long
				);
				function raiseNodataEvent(uint number, int16 short, uint32 long) {
					NodataEvent(number, short, long);
				}

				event DynamicEvent (
					string indexed IndexedString,
					bytes  indexed IndexedBytes,
					string NonIndexedString,
					bytes  NonIndexedBytes
				);
				function raiseDynamicEvent(string str, bytes blob) {
					DynamicEvent(str, blob, str, blob);
				}

				event FixedBytesEvent (
					bytes24 indexed IndexedBytes,
					bytes24 NonIndexedBytes
				);
				function raiseFixedBytesEvent(bytes24 blob) {
					FixedBytesEvent(blob, blob);
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b506109428061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061004a575f3560e01c8063528300ff1461004e578063630c31e21461006a5780636cc6b94014610086578063c7d116dd146100a2575b5f80fd5b610068600480360381019061006391906103ed565b6100be565b005b610084600480360381019061007f9190610558565b610127565b005b6100a0600480360381019061009b9190610611565b61017f565b005b6100bc60048036038101906100b791906106ab565b6101c5565b005b806040516100cc9190610767565b6040518091039020826040516100e291906107c1565b60405180910390207f3281fd4f5e152dd3385df49104a3f633706e21c9e80672e88d3bcddf33101f00848460405161011b929190610867565b60405180910390a35050565b811515838573ffffffffffffffffffffffffffffffffffffffff167f1f097de4289df643bd9c11011cc61367aa12983405c021056e706eb5ba1250c88460405161017191906108ab565b60405180910390a450505050565b8067ffffffffffffffff19167fcdc4c1b1aed5524ffb4198d7a5839a34712baef5fa06884fac7559f4a5854e0a826040516101ba91906108d3565b60405180910390a250565b8063ffffffff168260010b847f3ca7f3a77e5e6e15e781850bc82e32adfa378a2a609370db24b4d0fae10da2c960405160405180910390a4505050565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6102618261021b565b810181811067ffffffffffffffff821117156102805761027f61022b565b5b80604052505050565b5f610292610202565b905061029e8282610258565b919050565b5f67ffffffffffffffff8211156102bd576102bc61022b565b5b6102c68261021b565b9050602081019050919050565b828183375f83830152505050565b5f6102f36102ee846102a3565b610289565b90508281526020810184848401111561030f5761030e610217565b5b61031a8482856102d3565b509392505050565b5f82601f83011261033657610335610213565b5b81356103468482602086016102e1565b91505092915050565b5f67ffffffffffffffff8211156103695761036861022b565b5b6103728261021b565b9050602081019050919050565b5f61039161038c8461034f565b610289565b9050828152602081018484840111156103ad576103ac610217565b5b6103b88482856102d3565b509392505050565b5f82601f8301126103d4576103d3610213565b5b81356103e484826020860161037f565b91505092915050565b5f80604083850312156104035761040261020b565b5b5f83013567ffffffffffffffff8111156104205761041f61020f565b5b61042c85828601610322565b925050602083013567ffffffffffffffff81111561044d5761044c61020f565b5b610459858286016103c0565b9150509250929050565b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f61048c82610463565b9050919050565b61049c81610482565b81146104a6575f80fd5b50565b5f813590506104b781610493565b92915050565b5f819050919050565b6104cf816104bd565b81146104d9575f80fd5b50565b5f813590506104ea816104c6565b92915050565b5f8115159050919050565b610504816104f0565b811461050e575f80fd5b50565b5f8135905061051f816104fb565b92915050565b5f819050919050565b61053781610525565b8114610541575f80fd5b50565b5f813590506105528161052e565b92915050565b5f805f80608085870312156105705761056f61020b565b5b5f61057d878288016104a9565b945050602061058e878288016104dc565b935050604061059f87828801610511565b92505060606105b087828801610544565b91505092959194509250565b5f7fffffffffffffffffffffffffffffffffffffffffffffffff000000000000000082169050919050565b6105f0816105bc565b81146105fa575f80fd5b50565b5f8135905061060b816105e7565b92915050565b5f602082840312156106265761062561020b565b5b5f610633848285016105fd565b91505092915050565b5f8160010b9050919050565b6106518161063c565b811461065b575f80fd5b50565b5f8135905061066c81610648565b92915050565b5f63ffffffff82169050919050565b61068a81610672565b8114610694575f80fd5b50565b5f813590506106a581610681565b92915050565b5f805f606084860312156106c2576106c161020b565b5b5f6106cf86828701610544565b93505060206106e08682870161065e565b92505060406106f186828701610697565b9150509250925092565b5f81519050919050565b5f81905092915050565b5f5b8381101561072c578082015181840152602081019050610711565b5f8484015250505050565b5f610741826106fb565b61074b8185610705565b935061075b81856020860161070f565b80840191505092915050565b5f6107728284610737565b915081905092915050565b5f81519050919050565b5f81905092915050565b5f61079b8261077d565b6107a58185610787565b93506107b581856020860161070f565b80840191505092915050565b5f6107cc8284610791565b915081905092915050565b5f82825260208201905092915050565b5f6107f18261077d565b6107fb81856107d7565b935061080b81856020860161070f565b6108148161021b565b840191505092915050565b5f82825260208201905092915050565b5f610839826106fb565b610843818561081f565b935061085381856020860161070f565b61085c8161021b565b840191505092915050565b5f6040820190508181035f83015261087f81856107e7565b90508181036020830152610893818461082f565b90509392505050565b6108a581610525565b82525050565b5f6020820190506108be5f83018461089c565b92915050565b6108cd816105bc565b82525050565b5f6020820190506108e65f8301846108c4565b9291505056fea264697066735822122062f03612170df71898788030c1f07a0f01e8e68967f3ac3676d794c581735c0564687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"string","name":"IndexedString","type":"string"},{"indexed":true,"internalType":"bytes","name":"IndexedBytes","type":"bytes"},{"indexed":false,"internalType":"string","name":"NonIndexedString","type":"string"},{"indexed":false,"internalType":"bytes","name":"NonIndexedBytes","type":"bytes"}],"name":"DynamicEvent","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"bytes24","name":"IndexedBytes","type":"bytes24"},{"indexed":false,"internalType":"bytes24","name":"NonIndexedBytes","type":"bytes24"}],"name":"FixedBytesEvent","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"uint256","name":"Number","type":"uint256"},{"indexed":true,"internalType":"int16","name":"Short","type":"int16"},{"indexed":true,"internalType":"uint32","name":"Long","type":"uint32"}],"name":"NodataEvent","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"Addr","type":"address"},{"indexed":true,"internalType":"bytes32","name":"Id","type":"bytes32"},{"indexed":true,"internalType":"bool","name":"Flag","type":"bool"},{"indexed":false,"internalType":"uint256","name":"Value","type":"uint256"}],"name":"SimpleEvent","type":"event"},{"inputs":[{"internalType":"string","name":"str","type":"string"},{"internalType":"bytes","name":"blob","type":"bytes"}],"name":"raiseDynamicEvent","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bytes24","name":"blob","type":"bytes24"}],"name":"raiseFixedBytesEvent","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"number","type":"uint256"},{"internalType":"int16","name":"short","type":"int16"},{"internalType":"uint32","name":"long","type":"uint32"}],"name":"raiseNodataEvent","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"addr","type":"address"},{"internalType":"bytes32","name":"id","type":"bytes32"},{"internalType":"bool","name":"flag","type":"bool"},{"internalType":"uint256","name":"value","type":"uint256"}],"name":"raiseSimpleEvent","outputs":[],"stateMutability":"nonpayable","type":"function"}]`},
		`
			"math/big"
			"time"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/common"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy an eventer contract
			_, _, eventer, err := DeployEventer(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy eventer contract: %v", err)
			}
			sim.Commit()

			// Inject a few events into the contract, gradually more in each block
			for i := 1; i <= 3; i++ {
				for j := 1; j <= i; j++ {
					if _, err := eventer.RaiseSimpleEvent(auth, common.Address{byte(j)}, [32]byte{byte(j)}, true, big.NewInt(int64(10*i+j))); err != nil {
						t.Fatalf("block %d, event %d: raise failed: %v", i, j, err)
					}
				}
				sim.Commit()
			}
			// Test filtering for certain events and ensure they can be found
			sit, err := eventer.FilterSimpleEvent(nil, []common.Address{common.Address{1}, common.Address{3}}, [][32]byte{{byte(1)}, {byte(2)}, {byte(3)}}, []bool{true})
			if err != nil {
				t.Fatalf("failed to filter for simple events: %v", err)
			}
			defer sit.Close()

			sit.Next()
			if sit.Event.Value.Uint64() != 11 || !sit.Event.Flag {
				t.Errorf("simple log content mismatch: have %v, want {11, true}", sit.Event)
			}
			sit.Next()
			if sit.Event.Value.Uint64() != 21 || !sit.Event.Flag {
				t.Errorf("simple log content mismatch: have %v, want {21, true}", sit.Event)
			}
			sit.Next()
			if sit.Event.Value.Uint64() != 31 || !sit.Event.Flag {
				t.Errorf("simple log content mismatch: have %v, want {31, true}", sit.Event)
			}
			sit.Next()
			if sit.Event.Value.Uint64() != 33 || !sit.Event.Flag {
				t.Errorf("simple log content mismatch: have %v, want {33, true}", sit.Event)
			}

			if sit.Next() {
				t.Errorf("unexpected simple event found: %+v", sit.Event)
			}
			if err = sit.Error(); err != nil {
				t.Fatalf("simple event iteration failed: %v", err)
			}
			// Test raising and filtering for an event with no data component
			if _, err := eventer.RaiseNodataEvent(auth, big.NewInt(314), 141, 271); err != nil {
				t.Fatalf("failed to raise nodata event: %v", err)
			}
			sim.Commit()

			nit, err := eventer.FilterNodataEvent(nil, []*big.Int{big.NewInt(314)}, []int16{140, 141, 142}, []uint32{271})
			if err != nil {
				t.Fatalf("failed to filter for nodata events: %v", err)
			}
			defer nit.Close()

			if !nit.Next() {
				t.Fatalf("nodata log not found: %v", nit.Error())
			}
			if nit.Event.Number.Uint64() != 314 {
				t.Errorf("nodata log content mismatch: have %v, want 314", nit.Event.Number)
			}
			if nit.Next() {
				t.Errorf("unexpected nodata event found: %+v", nit.Event)
			}
			if err = nit.Error(); err != nil {
				t.Fatalf("nodata event iteration failed: %v", err)
			}
			// Test raising and filtering for events with dynamic indexed components
			if _, err := eventer.RaiseDynamicEvent(auth, "Hello", []byte("World")); err != nil {
				t.Fatalf("failed to raise dynamic event: %v", err)
			}
			sim.Commit()

			dit, err := eventer.FilterDynamicEvent(nil, []string{"Hi", "Hello", "Bye"}, [][]byte{[]byte("World")})
			if err != nil {
				t.Fatalf("failed to filter for dynamic events: %v", err)
			}
			defer dit.Close()

			if !dit.Next() {
				t.Fatalf("dynamic log not found: %v", dit.Error())
			}
			if dit.Event.NonIndexedString != "Hello" || string(dit.Event.NonIndexedBytes) != "World" ||	dit.Event.IndexedString != common.HexToHash("0x06b3dfaec148fb1bb2b066f10ec285e7c9bf402ab32aa78a5d38e34566810cd2") || dit.Event.IndexedBytes != common.HexToHash("0xf2208c967df089f60420785795c0a9ba8896b0f6f1867fa7f1f12ad6f79c1a18") {
				t.Errorf("dynamic log content mismatch: have %v, want {'0x06b3dfaec148fb1bb2b066f10ec285e7c9bf402ab32aa78a5d38e34566810cd2, '0xf2208c967df089f60420785795c0a9ba8896b0f6f1867fa7f1f12ad6f79c1a18', 'Hello', 'World'}", dit.Event)
			}
			if dit.Next() {
				t.Errorf("unexpected dynamic event found: %+v", dit.Event)
			}
			if err = dit.Error(); err != nil {
				t.Fatalf("dynamic event iteration failed: %v", err)
			}
			// Test raising and filtering for events with fixed bytes components
			var fblob [24]byte
			copy(fblob[:], []byte("Fixed Bytes"))

			if _, err := eventer.RaiseFixedBytesEvent(auth, fblob); err != nil {
				t.Fatalf("failed to raise fixed bytes event: %v", err)
			}
			sim.Commit()

			fit, err := eventer.FilterFixedBytesEvent(nil, [][24]byte{fblob})
			if err != nil {
				t.Fatalf("failed to filter for fixed bytes events: %v", err)
			}
			defer fit.Close()

			if !fit.Next() {
				t.Fatalf("fixed bytes log not found: %v", fit.Error())
			}
			if fit.Event.NonIndexedBytes != fblob || fit.Event.IndexedBytes != fblob {
				t.Errorf("fixed bytes log content mismatch: have %v, want {'%x', '%x'}", fit.Event, fblob, fblob)
			}
			if fit.Next() {
				t.Errorf("unexpected fixed bytes event found: %+v", fit.Event)
			}
			if err = fit.Error(); err != nil {
				t.Fatalf("fixed bytes event iteration failed: %v", err)
			}
			// Test subscribing to an event and raising it afterwards
			ch := make(chan *EventerSimpleEvent, 16)
			sub, err := eventer.WatchSimpleEvent(nil, ch, nil, nil, nil)
			if err != nil {
				t.Fatalf("failed to subscribe to simple events: %v", err)
			}
			if _, err := eventer.RaiseSimpleEvent(auth, common.Address{255}, [32]byte{255}, true, big.NewInt(255)); err != nil {
				t.Fatalf("failed to raise subscribed simple event: %v", err)
			}
			sim.Commit()

			select {
			case event := <-ch:
				if event.Value.Uint64() != 255 {
					t.Errorf("simple log content mismatch: have %v, want 255", event)
				}
			case <-time.After(250 * time.Millisecond):
				t.Fatalf("subscribed simple event didn't arrive")
			}
			// Unsubscribe from the event and make sure we're not delivered more
			sub.Unsubscribe()

			if _, err := eventer.RaiseSimpleEvent(auth, common.Address{254}, [32]byte{254}, true, big.NewInt(254)); err != nil {
				t.Fatalf("failed to raise subscribed simple event: %v", err)
			}
			sim.Commit()

			select {
			case event := <-ch:
				t.Fatalf("unsubscribed simple event arrived: %v", event)
			case <-time.After(250 * time.Millisecond):
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	{
		`DeeplyNestedArray`,
		`
		contract DeeplyNestedArray {
			uint64[3][4][5] public deepUint64Array;
			function storeDeepUintArray(uint64[3][4][5] memory arr) public {
				deepUint64Array = arr;
			}
			function retrieveDeepArray() public view returns (uint64[3][4][5] memory) {
				return deepUint64Array;
			}
		}
		`,
		[]string{`608060405234801561000f575f80fd5b50610a6d8061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c806334424855146100435780638ed4573a1461005f57806398ed18561461007d575b5f80fd5b61005d60048036038101906100589190610754565b6100ad565b005b6100676100c1565b6040516100749190610952565b60405180910390f35b6100976004803603810190610092919061099f565b6101a9565b6040516100a491906109fe565b60405180910390f35b805f9060056100bd9291906101fe565b5050565b6100c961024f565b5f600580602002604051908101604052809291905f905b828210156101a057838260040201600480602002604051908101604052809291905f905b8282101561018d57838201600380602002604051908101604052809291908260038015610179576020028201915f905b82829054906101000a900467ffffffffffffffff1667ffffffffffffffff16815260200190600801906020826007010492830192600103820291508084116101345790505b505050505081526020019060010190610104565b50505050815260200190600101906100e0565b50505050905090565b5f83600581106101b7575f80fd5b6004020182600481106101c8575f80fd5b0181600381106101d6575f80fd5b6004918282040191900660080292509250509054906101000a900467ffffffffffffffff1681565b826005600402810192821561023e579160200282015b8281111561023d5782518290600461022d92919061027c565b5091602001919060040190610214565b5b50905061024b91906102ca565b5090565b6040518060a001604052806005905b6102666102ed565b81526020019060019003908161025e5790505090565b82600481019282156102b9579160200282015b828111156102b8578251829060036102a892919061031a565b509160200191906001019061028f565b5b5090506102c691906103c7565b5090565b5b808211156102e9575f81816102e091906103ea565b506004016102cb565b5090565b60405180608001604052806004905b61030461042b565b8152602001906001900390816102fc5790505090565b82600380016004900481019282156103b6579160200282015f5b8382111561038057835183826101000a81548167ffffffffffffffff021916908367ffffffffffffffff1602179055509260200192600801602081600701049283019260010302610334565b80156103b45782816101000a81549067ffffffffffffffff0219169055600801602081600701049283019260010302610380565b505b5090506103c3919061044d565b5090565b5b808211156103e6575f81816103dd9190610468565b506001016103c8565b5090565b505f81816103f89190610468565b506001015f81816104099190610468565b506001015f818161041a9190610468565b506001015f6104299190610468565b565b6040518060600160405280600390602082028036833780820191505090505090565b5b80821115610464575f815f90555060010161044e565b5090565b505f9055565b5f604051905090565b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6104c58261047f565b810181811067ffffffffffffffff821117156104e4576104e361048f565b5b80604052505050565b5f6104f661046e565b905061050282826104bc565b919050565b5f67ffffffffffffffff8211156105215761052061048f565b5b602082029050919050565b5f80fd5b5f67ffffffffffffffff82111561054a5761054961048f565b5b602082029050919050565b5f67ffffffffffffffff82111561056f5761056e61048f565b5b602082029050919050565b5f67ffffffffffffffff82169050919050565b6105968161057a565b81146105a0575f80fd5b50565b5f813590506105b18161058d565b92915050565b5f6105c96105c484610555565b6104ed565b905080602084028301858111156105e3576105e261052c565b5b835b8181101561060c57806105f888826105a3565b8452602084019350506020810190506105e5565b5050509392505050565b5f82601f83011261062a5761062961047b565b5b60036106378482856105b7565b91505092915050565b5f61065261064d84610530565b6104ed565b9050806060840283018581111561066c5761066b61052c565b5b835b8181101561069557806106818882610616565b84526020840193505060608101905061066e565b5050509392505050565b5f82601f8301126106b3576106b261047b565b5b60046106c0848285610640565b91505092915050565b5f6106db6106d684610507565b6104ed565b90508061018084028301858111156106f6576106f561052c565b5b835b81811015610720578061070b888261069f565b845260208401935050610180810190506106f8565b5050509392505050565b5f82601f83011261073e5761073d61047b565b5b600561074b8482856106c9565b91505092915050565b5f610780828403121561076a57610769610477565b5b5f6107778482850161072a565b91505092915050565b5f60059050919050565b5f81905092915050565b5f819050919050565b5f60049050919050565b5f81905092915050565b5f819050919050565b5f60039050919050565b5f81905092915050565b5f819050919050565b6107e08161057a565b82525050565b5f6107f183836107d7565b60208301905092915050565b5f602082019050919050565b610812816107ba565b61081c81846107c4565b9250610827826107ce565b805f5b8381101561085757815161083e87826107e6565b9650610849836107fd565b92505060018101905061082a565b505050505050565b5f61086a8383610809565b60608301905092915050565b5f602082019050919050565b61088b8161079d565b61089581846107a7565b92506108a0826107b1565b805f5b838110156108d05781516108b7878261085f565b96506108c283610876565b9250506001810190506108a3565b505050505050565b5f6108e38383610882565b6101808301905092915050565b5f602082019050919050565b61090581610780565b61090f818461078a565b925061091a82610794565b805f5b8381101561094a57815161093187826108d8565b965061093c836108f0565b92505060018101905061091d565b505050505050565b5f610780820190506109665f8301846108fc565b92915050565b5f819050919050565b61097e8161096c565b8114610988575f80fd5b50565b5f8135905061099981610975565b92915050565b5f805f606084860312156109b6576109b5610477565b5b5f6109c38682870161098b565b93505060206109d48682870161098b565b92505060406109e58682870161098b565b9150509250925092565b6109f88161057a565b82525050565b5f602082019050610a115f8301846109ef565b9291505056fea264697066735822122062599b39d7cc6efca15edddc72c8760924a7b295d75b8b2efbcfd9889e90ff6764687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[{"internalType":"uint256","name":"","type":"uint256"},{"internalType":"uint256","name":"","type":"uint256"},{"internalType":"uint256","name":"","type":"uint256"}],"name":"deepUint64Array","outputs":[{"internalType":"uint64","name":"","type":"uint64"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"retrieveDeepArray","outputs":[{"internalType":"uint64[3][4][5]","name":"","type":"uint64[3][4][5]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint64[3][4][5]","name":"arr","type":"uint64[3][4][5]"}],"name":"storeDeepUintArray","outputs":[],"stateMutability":"nonpayable","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			//deploy the test contract
			_, _, testContract, err := DeployDeeplyNestedArray(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy test contract: %v", err)
			}

			// Finish deploy.
			sim.Commit()

			//Create coordinate-filled array, for testing purposes.
			testArr := [5][4][3]uint64{}
			for i := range 5 {
				testArr[i] = [4][3]uint64{}
				for j := range 4 {
					testArr[i][j] = [3]uint64{}
					for k := range 3 {
						//pack the coordinates, each array value will be unique, and can be validated easily.
						testArr[i][j][k] = uint64(i) << 16 | uint64(j) << 8 | uint64(k)
					}
				}
			}

			if _, err := testContract.StoreDeepUintArray(&bind.TransactOpts{
				From: auth.From,
				Signer: auth.Signer,
			}, testArr); err != nil {
				t.Fatalf("Failed to store nested array in test contract: %v", err)
			}

			sim.Commit()

			retrievedArr, err := testContract.RetrieveDeepArray(&bind.CallOpts{
				From: auth.From,
				Pending: false,
			})
			if err != nil {
				t.Fatalf("Failed to retrieve nested array from test contract: %v", err)
			}

			//quick check to see if contents were copied
			// (See accounts/abi/unpack_test.go for more extensive testing)
			if retrievedArr[4][3][2] != testArr[4][3][2] {
				t.Fatalf("Retrieved value does not match expected value! got: %d, expected: %d. %v", retrievedArr[4][3][2], testArr[4][3][2], err)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	{
		`CallbackParam`,
		`
			contract FunctionPointerTest {
				function test(function(uint256) external callback) external {
					callback(1);
				}
			}
		`,
		[]string{`608060405234801561000f575f80fd5b5061024a8061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c8063d7a5aba21461002d575b5f80fd5b61004760048036038101906100429190610161565b610049565b005b818160016040518263ffffffff1660e01b815260040161006991906101db565b5f604051808303815f87803b158015610080575f80fd5b505af1158015610092573d5f803e3d5ffd5b505050505050565b5f80fd5b5f7fffffffffffffffffffffffffffffffffffffffffffffffff000000000000000082169050919050565b5f6100d38261009e565b9050919050565b6100e3816100c9565b81146100ed575f80fd5b50565b5f813590506100fe816100da565b92915050565b5f8160201c9050919050565b5f8160401c9050919050565b5f8061012783610110565b925063ffffffff8316905061013b83610104565b9150915091565b5f8061015661015185856100f0565b61011c565b915091509250929050565b5f80602083850312156101775761017661009a565b5b5f61018485828601610142565b92509250509250929050565b5f819050919050565b5f819050919050565b5f819050919050565b5f6101c56101c06101bb84610190565b6101a2565b610199565b9050919050565b6101d5816101ab565b82525050565b5f6020820190506101ee5f8301846101cc565b9291505056fea264697066735822122079b8deb9359e34c2904b5fc67d1190a6f0c72f03695d9f679a391f03d46906cd64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs":[{"internalType":"function (uint256) external","name":"callback","type":"function"}],"name":"test","outputs":[],"stateMutability":"nonpayable","type":"function"}]`},
		`
			"strings"
		`,
		`
			if strings.Compare("test(function)", CallbackParamFuncSigs["d7a5aba2"]) != 0 {
				t.Fatalf("")
			}
		`,
		[]map[string]string{
			{
				"test(function)": "d7a5aba2",
			},
		},
		nil,
		nil,
		nil,
	},
	{
		`Tuple`,
		`
		pragma experimental ABIEncoderV2;

		contract Tuple {
			struct S { uint a; uint[] b; T[] c; }
			struct T { uint x; uint y; }
			struct P { uint8 x; uint8 y; }
			struct Q { uint16 x; uint16 y; }
			event TupleEvent(S a, T[2][] b, T[][2] c, S[] d, uint[] e);
			event TupleEvent2(P[]);

			function func1(S memory a, T[2][] memory b, T[][2] memory c, S[] memory d, uint[] memory e) public pure returns (S memory, T[2][] memory, T[][2] memory, S[] memory, uint[] memory) {
				return (a, b, c, d, e);
			}
			function func2(S memory a, T[2][] memory b, T[][2] memory c, S[] memory d, uint[] memory e) public {
				emit TupleEvent(a, b, c, d, e);
			}
			function func3(Q[] memory) public pure {} // call function, nothing to return
		}
		`,
		[]string{`608060405234801561000f575f80fd5b506110538061001d5f395ff3fe608060405234801561000f575f80fd5b506004361061003f575f3560e01c8063443c79b414610043578063d0062cdd14610077578063e4d9a43b14610093575b5f80fd5b61005d600480360381019061005891906107ca565b6100af565b60405161006e959493929190610dfe565b60405180910390f35b610091600480360381019061008c91906107ca565b6100e0565b005b6100ad60048036038101906100a89190610fb6565b610126565b005b6100b7610129565b60606100c1610149565b6060808989898989945094509450945094509550955095509550959050565b7f18d6e66efa53739ca6d13626f35ebc700b31cced3eddb50c70bbe9c082c6cd008585858585604051610117959493929190610dfe565b60405180910390a15050505050565b50565b60405180606001604052805f815260200160608152602001606081525090565b60405180604001604052806002905b60608152602001906001900390816101585790505090565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6101cb82610185565b810181811067ffffffffffffffff821117156101ea576101e9610195565b5b80604052505050565b5f6101fc610170565b905061020882826101c2565b919050565b5f80fd5b5f819050919050565b61022381610211565b811461022d575f80fd5b50565b5f8135905061023e8161021a565b92915050565b5f80fd5b5f67ffffffffffffffff82111561026257610261610195565b5b602082029050602081019050919050565b5f80fd5b5f61028961028484610248565b6101f3565b905080838252602082019050602084028301858111156102ac576102ab610273565b5b835b818110156102d557806102c18882610230565b8452602084019350506020810190506102ae565b5050509392505050565b5f82601f8301126102f3576102f2610244565b5b8135610303848260208601610277565b91505092915050565b5f67ffffffffffffffff82111561032657610325610195565b5b602082029050602081019050919050565b5f6040828403121561034c5761034b610181565b5b61035660406101f3565b90505f61036584828501610230565b5f83015250602061037884828501610230565b60208301525092915050565b5f6103966103918461030c565b6101f3565b905080838252602082019050604084028301858111156103b9576103b8610273565b5b835b818110156103e257806103ce8882610337565b8452602084019350506040810190506103bb565b5050509392505050565b5f82601f830112610400576103ff610244565b5b8135610410848260208601610384565b91505092915050565b5f6060828403121561042e5761042d610181565b5b61043860606101f3565b90505f61044784828501610230565b5f83015250602082013567ffffffffffffffff81111561046a5761046961020d565b5b610476848285016102df565b602083015250604082013567ffffffffffffffff81111561049a5761049961020d565b5b6104a6848285016103ec565b60408301525092915050565b5f67ffffffffffffffff8211156104cc576104cb610195565b5b602082029050602081019050919050565b5f67ffffffffffffffff8211156104f7576104f6610195565b5b602082029050919050565b5f61051461050f846104dd565b6101f3565b9050806040840283018581111561052e5761052d610273565b5b835b8181101561055757806105438882610337565b845260208401935050604081019050610530565b5050509392505050565b5f82601f83011261057557610574610244565b5b6002610582848285610502565b91505092915050565b5f61059d610598846104b2565b6101f3565b905080838252602082019050608084028301858111156105c0576105bf610273565b5b835b818110156105e957806105d58882610561565b8452602084019350506080810190506105c2565b5050509392505050565b5f82601f83011261060757610606610244565b5b813561061784826020860161058b565b91505092915050565b5f67ffffffffffffffff82111561063a57610639610195565b5b602082029050919050565b5f61065761065284610620565b6101f3565b9050806020840283018581111561067157610670610273565b5b835b818110156106b857803567ffffffffffffffff81111561069657610695610244565b5b8086016106a389826103ec565b85526020850194505050602081019050610673565b5050509392505050565b5f82601f8301126106d6576106d5610244565b5b60026106e3848285610645565b91505092915050565b5f67ffffffffffffffff82111561070657610705610195565b5b602082029050602081019050919050565b5f610729610724846106ec565b6101f3565b9050808382526020820190506020840283018581111561074c5761074b610273565b5b835b8181101561079357803567ffffffffffffffff81111561077157610770610244565b5b80860161077e8982610419565b8552602085019450505060208101905061074e565b5050509392505050565b5f82601f8301126107b1576107b0610244565b5b81356107c1848260208601610717565b91505092915050565b5f805f805f60a086880312156107e3576107e2610179565b5b5f86013567ffffffffffffffff811115610800576107ff61017d565b5b61080c88828901610419565b955050602086013567ffffffffffffffff81111561082d5761082c61017d565b5b610839888289016105f3565b945050604086013567ffffffffffffffff81111561085a5761085961017d565b5b610866888289016106c2565b935050606086013567ffffffffffffffff8111156108875761088661017d565b5b6108938882890161079d565b925050608086013567ffffffffffffffff8111156108b4576108b361017d565b5b6108c0888289016102df565b9150509295509295909350565b6108d681610211565b82525050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b5f61091083836108cd565b60208301905092915050565b5f602082019050919050565b5f610932826108dc565b61093c81856108e6565b9350610947836108f6565b805f5b8381101561097757815161095e8882610905565b97506109698361091c565b92505060018101905061094a565b5085935050505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b604082015f8201516109c15f8501826108cd565b5060208201516109d460208501826108cd565b50505050565b5f6109e583836109ad565b60408301905092915050565b5f602082019050919050565b5f610a0782610984565b610a11818561098e565b9350610a1c8361099e565b805f5b83811015610a4c578151610a3388826109da565b9750610a3e836109f1565b925050600181019050610a1f565b5085935050505092915050565b5f606083015f830151610a6e5f8601826108cd565b5060208301518482036020860152610a868282610928565b91505060408301518482036040860152610aa082826109fd565b9150508091505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b5f60029050919050565b5f81905092915050565b5f819050919050565b5f602082019050919050565b610b0881610ad6565b610b128184610ae0565b9250610b1d82610aea565b805f5b83811015610b4d578151610b3487826109da565b9650610b3f83610af3565b925050600181019050610b20565b505050505050565b5f610b608383610aff565b60808301905092915050565b5f602082019050919050565b5f610b8282610aad565b610b8c8185610ab7565b9350610b9783610ac7565b805f5b83811015610bc7578151610bae8882610b55565b9750610bb983610b6c565b925050600181019050610b9a565b5085935050505092915050565b5f60029050919050565b5f81905092915050565b5f819050919050565b5f610bfc83836109fd565b905092915050565b5f602082019050919050565b5f610c1a82610bd4565b610c248185610bde565b935083602082028501610c3685610be8565b805f5b85811015610c715784840389528151610c528582610bf1565b9450610c5d83610c04565b925060208a01995050600181019050610c39565b50829750879550505050505092915050565b5f81519050919050565b5f82825260208201905092915050565b5f819050602082019050919050565b5f606083015f830151610cc15f8601826108cd565b5060208301518482036020860152610cd98282610928565b91505060408301518482036040860152610cf382826109fd565b9150508091505092915050565b5f610d0b8383610cac565b905092915050565b5f602082019050919050565b5f610d2982610c83565b610d338185610c8d565b935083602082028501610d4585610c9d565b805f5b85811015610d805784840389528151610d618582610d00565b9450610d6c83610d13565b925060208a01995050600181019050610d48565b50829750879550505050505092915050565b5f82825260208201905092915050565b5f610dac826108dc565b610db68185610d92565b9350610dc1836108f6565b805f5b83811015610df1578151610dd88882610905565b9750610de38361091c565b925050600181019050610dc4565b5085935050505092915050565b5f60a0820190508181035f830152610e168188610a59565b90508181036020830152610e2a8187610b78565b90508181036040830152610e3e8186610c10565b90508181036060830152610e528185610d1f565b90508181036080830152610e668184610da2565b90509695505050505050565b5f67ffffffffffffffff821115610e8c57610e8b610195565b5b602082029050602081019050919050565b5f61ffff82169050919050565b610eb381610e9d565b8114610ebd575f80fd5b50565b5f81359050610ece81610eaa565b92915050565b5f60408284031215610ee957610ee8610181565b5b610ef360406101f3565b90505f610f0284828501610ec0565b5f830152506020610f1584828501610ec0565b60208301525092915050565b5f610f33610f2e84610e72565b6101f3565b90508083825260208201905060408402830185811115610f5657610f55610273565b5b835b81811015610f7f5780610f6b8882610ed4565b845260208401935050604081019050610f58565b5050509392505050565b5f82601f830112610f9d57610f9c610244565b5b8135610fad848260208601610f21565b91505092915050565b5f60208284031215610fcb57610fca610179565b5b5f82013567ffffffffffffffff811115610fe857610fe761017d565b5b610ff484828501610f89565b9150509291505056fea2646970667358221220ef150d4a387e2aabbabc0c94e6d25b8dd65c43a46e0007abc51e87abf7463c4d64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"anonymous":false,"inputs":[{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"indexed":false,"internalType":"struct Tuple.S","name":"a","type":"tuple"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"indexed":false,"internalType":"struct Tuple.T[2][]","name":"b","type":"tuple[2][]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"indexed":false,"internalType":"struct Tuple.T[][2]","name":"c","type":"tuple[][2]"},{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"indexed":false,"internalType":"struct Tuple.S[]","name":"d","type":"tuple[]"},{"indexed":false,"internalType":"uint256[]","name":"e","type":"uint256[]"}],"name":"TupleEvent","type":"event"},{"anonymous":false,"inputs":[{"components":[{"internalType":"uint8","name":"x","type":"uint8"},{"internalType":"uint8","name":"y","type":"uint8"}],"indexed":false,"internalType":"struct Tuple.P[]","name":"","type":"tuple[]"}],"name":"TupleEvent2","type":"event"},{"inputs":[{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"internalType":"struct Tuple.S","name":"a","type":"tuple"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[2][]","name":"b","type":"tuple[2][]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[][2]","name":"c","type":"tuple[][2]"},{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"internalType":"struct Tuple.S[]","name":"d","type":"tuple[]"},{"internalType":"uint256[]","name":"e","type":"uint256[]"}],"name":"func1","outputs":[{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"internalType":"struct Tuple.S","name":"","type":"tuple"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[2][]","name":"","type":"tuple[2][]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[][2]","name":"","type":"tuple[][2]"},{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"internalType":"struct Tuple.S[]","name":"","type":"tuple[]"},{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"pure","type":"function"},{"inputs":[{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"internalType":"struct Tuple.S","name":"a","type":"tuple"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[2][]","name":"b","type":"tuple[2][]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[][2]","name":"c","type":"tuple[][2]"},{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256[]","name":"b","type":"uint256[]"},{"components":[{"internalType":"uint256","name":"x","type":"uint256"},{"internalType":"uint256","name":"y","type":"uint256"}],"internalType":"struct Tuple.T[]","name":"c","type":"tuple[]"}],"internalType":"struct Tuple.S[]","name":"d","type":"tuple[]"},{"internalType":"uint256[]","name":"e","type":"uint256[]"}],"name":"func2","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"components":[{"internalType":"uint16","name":"x","type":"uint16"},{"internalType":"uint16","name":"y","type":"uint16"}],"internalType":"struct Tuple.Q[]","name":"","type":"tuple[]"}],"name":"func3","outputs":[],"stateMutability":"pure","type":"function"}]`},
		`
			"math/big"
			"reflect"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			_, _, contract, err := DeployTuple(auth, sim)
			if err != nil {
				t.Fatalf("deploy contract failed %v", err)
			}
			sim.Commit()

			check := func(a, b any, errMsg string) {
				if !reflect.DeepEqual(a, b) {
					t.Fatal(errMsg)
				}
			}

			a := TupleS{
				A: big.NewInt(1),
				B: []*big.Int{big.NewInt(2), big.NewInt(3)},
				C: []TupleT{
					{
						X: big.NewInt(4),
						Y: big.NewInt(5),
					},
					{
						X: big.NewInt(6),
						Y: big.NewInt(7),
					},
				},
			}

			b := [][2]TupleT{
				{
					{
						X: big.NewInt(8),
						Y: big.NewInt(9),
					},
					{
						X: big.NewInt(10),
						Y: big.NewInt(11),
					},
				},
			}

			c := [2][]TupleT{
				{
					{
						X: big.NewInt(12),
						Y: big.NewInt(13),
					},
					{
						X: big.NewInt(14),
						Y: big.NewInt(15),
					},
				},
				{
					{
						X: big.NewInt(16),
						Y: big.NewInt(17),
					},
				},
			}

			d := []TupleS{a}

			e := []*big.Int{big.NewInt(18), big.NewInt(19)}
			ret1, ret2, ret3, ret4, ret5, err := contract.Func1(nil, a, b, c, d, e)
			if err != nil {
				t.Fatalf("invoke contract failed, err %v", err)
			}
			check(ret1, a, "ret1 mismatch")
			check(ret2, b, "ret2 mismatch")
			check(ret3, c, "ret3 mismatch")
			check(ret4, d, "ret4 mismatch")
			check(ret5, e, "ret5 mismatch")

			_, err = contract.Func2(auth, a, b, c, d, e)
			if err != nil {
				t.Fatalf("invoke contract failed, err %v", err)
			}
			sim.Commit()

			iter, err := contract.FilterTupleEvent(nil)
			if err != nil {
				t.Fatalf("failed to create event filter, err %v", err)
			}
			defer iter.Close()

			iter.Next()
			check(iter.Event.A, a, "field1 mismatch")
			check(iter.Event.B, b, "field2 mismatch")
			check(iter.Event.C, c, "field3 mismatch")
			check(iter.Event.D, d, "field4 mismatch")
			check(iter.Event.E, e, "field5 mismatch")

			err = contract.Func3(nil, nil)
			if err != nil {
				t.Fatalf("failed to call function which has no return, err %v", err)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	{
		`UseLibrary`,
		`
		library Math {
			function add(uint a, uint b) public view returns(uint) {
				return a + b;
			}
		}

		contract UseLibrary {
			function add (uint c, uint d) public view returns(uint) {
				return Math.add(c,d);
			}
		}
		`,
		[]string{
			// Bytecode for the UseLibrary contract
			`608060405234801561000f575f80fd5b506102468061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c8063771602f71461002d575b5f80fd5b61004760048036038101906100429190610115565b61005d565b6040516100549190610162565b60405180910390f35b5f73__$73f3a9ed08128f832e9c5f1bc1ceee2289$__63771602f784846040518363ffffffff1660e01b815260040161009792919061018a565b602060405180830381865af41580156100b2573d5f803e3d5ffd5b505050506040513d601f19601f820116820180604052508101906100d691906101c5565b905092915050565b5f80fd5b5f819050919050565b6100f4816100e2565b81146100fe575f80fd5b50565b5f8135905061010f816100eb565b92915050565b5f806040838503121561012b5761012a6100de565b5b5f61013885828601610101565b925050602061014985828601610101565b9150509250929050565b61015c816100e2565b82525050565b5f6020820190506101755f830184610153565b92915050565b610184816100e2565b82525050565b5f60408201905061019d5f83018561017b565b6101aa602083018461017b565b9392505050565b5f815190506101bf816100eb565b92915050565b5f602082840312156101da576101d96100de565b5b5f6101e7848285016101b1565b9150509291505056fea2646970667358221220c22b312014c994e385445b551f8558ea38b5b988fed9a09fe33dff2303f1bf7564687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`,
			// Bytecode for the Math contract
			`6101d061004e600b8282823980515f1a607314610042577f4e487b71000000000000000000000000000000000000000000000000000000005f525f60045260245ffd5b305f52607381538281f3fe7300000000000000000000000000000000000000003014608060405260043610610034575f3560e01c8063771602f714610038575b5f80fd5b610052600480360381019061004d91906100b4565b610068565b60405161005f9190610101565b60405180910390f35b5f81836100759190610147565b905092915050565b5f80fd5b5f819050919050565b61009381610081565b811461009d575f80fd5b50565b5f813590506100ae8161008a565b92915050565b5f80604083850312156100ca576100c961007d565b5b5f6100d7858286016100a0565b92505060206100e8858286016100a0565b9150509250929050565b6100fb81610081565b82525050565b5f6020820190506101145f8301846100f2565b92915050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f61015182610081565b915061015c83610081565b92508282019050808211156101745761017361011a565b5b9291505056fea2646970667358221220ef32547ce2999432a959e47fd64f25a2e309a098f84d2f19610a22aa8fef4cb264687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`,
		},
		[]string{
			`[{"inputs":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256","name":"b","type":"uint256"}],"name":"add","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`,
			`[{"inputs":[{"internalType":"uint256","name":"c","type":"uint256"},{"internalType":"uint256","name":"d","type":"uint256"}],"name":"add","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`,
		},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			//deploy the test contract
			_, _, testContract, err := DeployUseLibrary(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy test contract: %v", err)
			}

			// Finish deploy.
			sim.Commit()

			// Check that the library contract has been deployed
			// by calling the contract's add function.
			res, err := testContract.Add(&bind.CallOpts{
				From: auth.From,
				Pending: false,
			}, big.NewInt(1), big.NewInt(2))
			if err != nil {
				t.Fatalf("Failed to call linked contract: %v", err)
			}
			if res.Cmp(big.NewInt(3)) != 0 {
				t.Fatalf("Add did not return the correct result: %d != %d", res, 3)
			}
		`,
		nil,
		map[string]string{
			"73f3a9ed08128f832e9c5f1bc1ceee2289": "Math",
		},
		nil,
		[]string{"UseLibrary", "Math"},
	},
	{
		"Overload",
		`
		contract overload {
			mapping(address => uint256) balances;

			event bar(uint256 i);
			event bar(uint256 i, uint256 j);

			function foo(uint256 i) public {
				emit bar(i);
			}
			function foo(uint256 i, uint256 j) public {
				emit bar(i, j);
			}
		}
		`,
		[]string{`608060405234801561000f575f80fd5b5061022c8061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c806304bc52f8146100385780632fbebd3814610054575b5f80fd5b610052600480360381019061004d919061011e565b610070565b005b61006e6004803603810190610069919061015c565b6100ad565b005b7fae42e9514233792a47a1e4554624e83fe852228e1503f63cd383e8a431f4f46d82826040516100a1929190610196565b60405180910390a15050565b7f0423a1321222a0a8716c22b92fac42d85a45a612b696a461784d9fa537c81e5c816040516100dc91906101bd565b60405180910390a150565b5f80fd5b5f819050919050565b6100fd816100eb565b8114610107575f80fd5b50565b5f81359050610118816100f4565b92915050565b5f8060408385031215610134576101336100e7565b5b5f6101418582860161010a565b92505060206101528582860161010a565b9150509250929050565b5f60208284031215610171576101706100e7565b5b5f61017e8482850161010a565b91505092915050565b610190816100eb565b82525050565b5f6040820190506101a95f830185610187565b6101b66020830184610187565b9392505050565b5f6020820190506101d05f830184610187565b9291505056fea2646970667358221220e0259de0befeaa65d85e4290151e279c8ca4c8c761ce6bdc33f46ec1ef42560e64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"i","type":"uint256"}],"name":"bar","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"i","type":"uint256"},{"indexed":false,"internalType":"uint256","name":"j","type":"uint256"}],"name":"bar","type":"event"},{"inputs":[{"internalType":"uint256","name":"i","type":"uint256"},{"internalType":"uint256","name":"j","type":"uint256"}],"name":"foo","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256","name":"i","type":"uint256"}],"name":"foo","outputs":[],"stateMutability":"nonpayable","type":"function"}]`},
		`
			"math/big"
			"time"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Initialize test accounts
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// deploy the test contract
			_, _, contract, err := DeployOverload(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy contract: %v", err)
			}
			// Finish deploy.
			sim.Commit()

			resCh, stopCh := make(chan uint64), make(chan struct{})

			go func() {
				barSink := make(chan *OverloadBar)
				sub, _ := contract.WatchBar(nil, barSink)
				defer sub.Unsubscribe()

				bar0Sink := make(chan *OverloadBar0)
				sub0, _ := contract.WatchBar0(nil, bar0Sink)
				defer sub0.Unsubscribe()

				for {
					select {
					case ev := <-barSink:
						resCh <- ev.I.Uint64()
					case ev := <-bar0Sink:
						resCh <- ev.I.Uint64() + ev.J.Uint64()
					case <-stopCh:
						return
					}
				}
			}()
			contract.Foo(auth, big.NewInt(1), big.NewInt(2))
			sim.Commit()
			select {
			case n := <-resCh:
				if n != 3 {
					t.Fatalf("Invalid bar0 event")
				}
			case <-time.NewTimer(3 * time.Second).C:
				t.Fatalf("Wait bar0 event timeout")
			}

			contract.Foo0(auth, big.NewInt(1))
			sim.Commit()
			select {
			case n := <-resCh:
				if n != 1 {
					t.Fatalf("Invalid bar event")
				}
			case <-time.NewTimer(3 * time.Second).C:
				t.Fatalf("Wait bar event timeout")
			}
			close(stopCh)
		`,
		nil,
		nil,
		nil,
		nil,
	},
	{
		"IdentifierCollision",
		`
		contract IdentifierCollision {
			uint public _myVar;

			function MyVar() public view returns (uint) {
				return _myVar;
			}
		}
		`,
		[]string{"608060405234801561000f575f80fd5b5060f88061001c5f395ff3fe6080604052348015600e575f80fd5b50600436106030575f3560e01c806301ad4d871460345780634ef1f0ad14604e575b5f80fd5b603a6068565b60405160459190608b565b60405180910390f35b6054606d565b604051605f9190608b565b60405180910390f35b5f5481565b5f8054905090565b5f819050919050565b6085816075565b82525050565b5f602082019050609c5f830184607e565b9291505056fea26469706673582212208b719183e23fa86666beb8980fe633305b283bbfb362f2b8475ca9fd476dfec164687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		[]string{`[{"inputs":[],"name":"MyVar","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"_myVar","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/core"
		`,
		`
			// Initialize test accounts
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			addr := wallet.GetAddress()

			// Deploy registrar contract
			sim := backends.NewSimulatedBackend(core.GenesisAlloc{addr: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			transactOpts, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
			_, _, _, err := DeployIdentifierCollision(transactOpts, sim)
			if err != nil {
				t.Fatalf("failed to deploy contract: %v", err)
			}
		`,
		nil,
		nil,
		map[string]string{"_myVar": "pubVar"}, // alias MyVar to PubVar
		nil,
	},
	{
		"MultiContracts",
		`
		pragma experimental ABIEncoderV2;

		library ExternalLib {
			struct SharedStruct{
				uint256 f1;
				bytes32 f2;
			}
		}

		contract ContractOne {
			function foo(ExternalLib.SharedStruct memory s) pure public {
				// Do stuff
			}
		}

		contract ContractTwo {
			function bar(ExternalLib.SharedStruct memory s) pure public {
				// Do stuff
			}
		}
		`,
		[]string{
			`608060405234801561000f575f80fd5b506102198061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c80639d8a8ba81461002d575b5f80fd5b61004760048036038101906100429190610198565b610049565b005b50565b5f604051905090565b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6100a38261005d565b810181811067ffffffffffffffff821117156100c2576100c161006d565b5b80604052505050565b5f6100d461004c565b90506100e0828261009a565b919050565b5f819050919050565b6100f7816100e5565b8114610101575f80fd5b50565b5f81359050610112816100ee565b92915050565b5f819050919050565b61012a81610118565b8114610134575f80fd5b50565b5f8135905061014581610121565b92915050565b5f604082840312156101605761015f610059565b5b61016a60406100cb565b90505f61017984828501610104565b5f83015250602061018c84828501610137565b60208301525092915050565b5f604082840312156101ad576101ac610055565b5b5f6101ba8482850161014b565b9150509291505056fea26469706673582212203ebf80a031ed8a68378ec3782c9603edc986df66f06842cfcf719dee5ffc823764687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`,
			`608060405234801561000f575f80fd5b506102198061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c8063db8ba08c1461002d575b5f80fd5b61004760048036038101906100429190610198565b610049565b005b50565b5f604051905090565b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6100a38261005d565b810181811067ffffffffffffffff821117156100c2576100c161006d565b5b80604052505050565b5f6100d461004c565b90506100e0828261009a565b919050565b5f819050919050565b6100f7816100e5565b8114610101575f80fd5b50565b5f81359050610112816100ee565b92915050565b5f819050919050565b61012a81610118565b8114610134575f80fd5b50565b5f8135905061014581610121565b92915050565b5f604082840312156101605761015f610059565b5b61016a60406100cb565b90505f61017984828501610104565b5f83015250602061018c84828501610137565b60208301525092915050565b5f604082840312156101ad576101ac610055565b5b5f6101ba8482850161014b565b9150509291505056fea2646970667358221220734f04499bfcf08a9c6b4768fe3343caf3fd7cec4eae12a1cc7a32663077527d64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`,
			`6075604b600b8282823980515f1a607314603f577f4e487b71000000000000000000000000000000000000000000000000000000005f525f60045260245ffd5b305f52607381538281f3fe730000000000000000000000000000000000000000301460806040525f80fdfea2646970667358221220e30df4ccb8c92c1a11b15ffe8bab7a14c8763dc57488a940585a87e7012f170964687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`,
		},
		[]string{
			`[{"inputs":[{"components":[{"internalType":"uint256","name":"f1","type":"uint256"},{"internalType":"bytes32","name":"f2","type":"bytes32"}],"internalType":"struct ExternalLib.SharedStruct","name":"s","type":"tuple"}],"name":"foo","outputs":[],"stateMutability":"pure","type":"function"}]`,
			`[{"inputs":[{"components":[{"internalType":"uint256","name":"f1","type":"uint256"},{"internalType":"bytes32","name":"f2","type":"bytes32"}],"internalType":"struct ExternalLib.SharedStruct","name":"s","type":"tuple"}],"name":"bar","outputs":[],"stateMutability":"pure","type":"function"}]`,
			`[]`,
		},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/core"
		`,
		`
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			addr := wallet.GetAddress()

			// Deploy registrar contract
			sim := backends.NewSimulatedBackend(core.GenesisAlloc{addr: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			transactOpts, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
			_, _, c1, err := DeployContractOne(transactOpts, sim)
			if err != nil {
				t.Fatal("Failed to deploy contract")
			}
			sim.Commit()
			err = c1.Foo(nil, ExternalLibSharedStruct{
				F1: big.NewInt(100),
				F2: [32]byte{0x01, 0x02, 0x03},
			})
			if err != nil {
				t.Fatal("Failed to invoke function")
			}
			_, _, c2, err := DeployContractTwo(transactOpts, sim)
			if err != nil {
				t.Fatal("Failed to deploy contract")
			}
			sim.Commit()
			err = c2.Bar(nil, ExternalLibSharedStruct{
				F1: big.NewInt(100),
				F2: [32]byte{0x01, 0x02, 0x03},
			})
			if err != nil {
				t.Fatal("Failed to invoke function")
			}
		`,
		nil,
		nil,
		nil,
		[]string{"ContractOne", "ContractTwo", "ExternalLib"},
	},
	// Test the existence of the free retrieval calls
	{
		`PureAndView`,
		`
		contract PureAndView {
			function PureFunc() public pure returns (uint) {
				return 42;
			}
			function ViewFunc() public view returns (uint) {
				return block.number;
			}
		}
		`,
		[]string{`608060405234801561000f575f80fd5b5060fa8061001c5f395ff3fe6080604052348015600e575f80fd5b50600436106030575f3560e01c806376b5686a146034578063bb38c66c14604e575b5f80fd5b603a6068565b60405160459190608d565b60405180910390f35b6054606f565b604051605f9190608d565b60405180910390f35b5f43905090565b5f602a905090565b5f819050919050565b6087816077565b82525050565b5f602082019050609e5f8301846080565b9291505056fea2646970667358221220bb067431dcd0bd371a5298262c594aceb2b057aba02a9b71bc27d5013e05973564687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		[]string{`[{"inputs": [],"name": "PureFunc","outputs": [{"internalType": "uint256","name": "","type": "uint256"}],"stateMutability": "pure","type": "function"},{"inputs": [],"name": "ViewFunc","outputs": [{"internalType": "uint256","name": "","type": "uint256"}],"stateMutability": "view","type": "function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			// Generate a new random account and a funded simulator
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			auth, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{auth.From: {Balance: big.NewInt(10000000000000000)}}, 10000000)
			defer sim.Close()

			// Deploy a tester contract and execute a structured call on it
			_, _, pav, err := DeployPureAndView(auth, sim)
			if err != nil {
				t.Fatalf("Failed to deploy PureAndView contract: %v", err)
			}
			sim.Commit()

			// This test the existence of the free retreiver call for view and pure functions
			if num, err := pav.PureFunc(nil); err != nil {
				t.Fatalf("Failed to call anonymous field retriever: %v", err)
			} else if num.Cmp(big.NewInt(42)) != 0 {
				t.Fatalf("Retrieved value mismatch: have %v, want %v", num, 42)
			}
			if num, err := pav.ViewFunc(nil); err != nil {
				t.Fatalf("Failed to call anonymous field retriever: %v", err)
			} else if num.Cmp(big.NewInt(1)) != 0 {
				t.Fatalf("Retrieved value mismatch: have %v, want %v", num, 1)
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test fallback separation
	{
		`NewFallbacks`,
		`
		contract NewFallbacks {
			event Fallback(bytes data);
			fallback() external {
				emit Fallback(msg.data);
			}

			event Received(address addr, uint value);
			receive() external payable {
				emit Received(msg.sender, msg.value);
			}
		}
		`,
		[]string{"608060405234801561000f575f80fd5b506101db8061001d5f395ff3fe608060405236610044577f88a5966d370b9919b20f3e2c13ff65706f196a4e32cc2c12bf57088f88525874333460405161003a9291906100e2565b60405180910390a1005b34801561004f575f80fd5b507f9043988963722edecc2099c75b0af0ff76af14ffca42ed6bce059a20a2a9f9865f36604051610081929190610163565b60405180910390a1005b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6100b48261008b565b9050919050565b6100c4816100aa565b82525050565b5f819050919050565b6100dc816100ca565b82525050565b5f6040820190506100f55f8301856100bb565b61010260208301846100d3565b9392505050565b5f82825260208201905092915050565b828183375f83830152505050565b5f601f19601f8301169050919050565b5f6101428385610109565b935061014f838584610119565b61015883610127565b840190509392505050565b5f6020820190508181035f83015261017c818486610137565b9050939250505056fea26469706673582212203b5f16d8e34da3ef6f53ef7c029ee95872f622ae369a2a67f3c7c67d8d800bd964687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		[]string{`[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"bytes","name":"data","type":"bytes"}],"name":"Fallback","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"addr","type":"address"},{"indexed":false,"internalType":"uint256","name":"value","type":"uint256"}],"name":"Received","type":"event"},{"stateMutability":"nonpayable","type":"fallback"},{"stateMutability":"payable","type":"receive"}]`},
		`
			"bytes"
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
		`,
		`
			wallet, _ := wallet.Generate(wallet.ML_DSA_87)
			addr := wallet.GetAddress()

			sim := backends.NewSimulatedBackend(core.GenesisAlloc{addr: {Balance: big.NewInt(10000000000000000)}}, 1000000)
			defer sim.Close()

			opts, _ := bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
			_, _, c, err := DeployNewFallbacks(opts, sim)
			if err != nil {
				t.Fatalf("Failed to deploy contract: %v", err)
			}
			sim.Commit()

			// Test receive function
			opts.Value = big.NewInt(100)
			c.Receive(opts)
			sim.Commit()

			var gotEvent bool
			iter, _ := c.FilterReceived(nil)
			defer iter.Close()
			for iter.Next() {
				if iter.Event.Addr != addr {
					t.Fatal("Msg.sender mismatch")
				}
				if iter.Event.Value.Uint64() != 100 {
					t.Fatal("Msg.value mismatch")
				}
				gotEvent = true
				break
			}
			if !gotEvent {
				t.Fatal("Expect to receive event emitted by receive")
			}

			// Test fallback function
			gotEvent = false
			opts.Value = nil
			calldata := []byte{0x01, 0x02, 0x03}
			c.Fallback(opts, calldata)
			sim.Commit()

			iter2, _ := c.FilterFallback(nil)
			defer iter2.Close()
			for iter2.Next() {
				if !bytes.Equal(iter2.Event.Data, calldata) {
					t.Fatal("calldata mismatch")
				}
				gotEvent = true
				break
			}
			if !gotEvent {
				t.Fatal("Expect to receive event emitted by fallback")
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test resolving single struct argument
	{
		`NewSingleStructArgument`,
		`
			contract NewSingleStructArgument {
				struct MyStruct{
					uint256 a;
					uint256 b;
				}
				event StructEvent(MyStruct s);
				function TestEvent() public {
					emit StructEvent(MyStruct({a: 1, b: 2}));
				}
			}
		`,
		[]string{"608060405234801561000f575f80fd5b5061012b8061001d5f395ff3fe6080604052348015600e575f80fd5b50600436106026575f3560e01c806324ec1d3f14602a575b5f80fd5b60306032565b005b7fb4b2ff75e30cb4317eaae16dd8a187dd89978df17565104caa6c2797caae27d460405180604001604052806001815260200160028152506040516075919060be565b60405180910390a1565b5f819050919050565b608f81607f565b82525050565b604082015f82015160a75f8501826088565b50602082015160b860208501826088565b50505050565b5f60408201905060cf5f8301846095565b9291505056fea264697066735822122019428c09e3c97b5253e0fac09985e19692549fdb37203476888863a598c2079164687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		[]string{`[{"anonymous":false,"inputs":[{"components":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256","name":"b","type":"uint256"}],"indexed":false,"internalType":"struct NewSingleStructArgument.MyStruct","name":"s","type":"tuple"}],"name":"StructEvent","type":"event"},{"inputs":[],"name":"TestEvent","outputs":[],"stateMutability":"nonpayable","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/qrl/qrlconfig"
		`,
		`
			var (
				wallet, _  = wallet.Generate(wallet.ML_DSA_87)
				user, _ = bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
				sim     = backends.NewSimulatedBackend(core.GenesisAlloc{user.From: {Balance: big.NewInt(1000000000000000000)}}, qrlconfig.Defaults.Miner.GasCeil)
			)
			defer sim.Close()

			_, _, d, err := DeployNewSingleStructArgument(user, sim)
			if err != nil {
				t.Fatalf("Failed to deploy contract %v", err)
			}
			sim.Commit()

			_, err = d.TestEvent(user)
			if err != nil {
				t.Fatalf("Failed to call contract %v", err)
			}
			sim.Commit()

			it, err := d.FilterStructEvent(nil)
			if err != nil {
				t.Fatalf("Failed to filter contract event %v", err)
			}
			var count int
			for it.Next() {
				if it.Event.S.A.Cmp(big.NewInt(1)) != 0 {
					t.Fatal("Unexpected contract event")
				}
				if it.Event.S.B.Cmp(big.NewInt(2)) != 0 {
					t.Fatal("Unexpected contract event")
				}
				count += 1
			}
			if count != 1 {
				t.Fatal("Unexpected contract event number")
			}
		`,
		nil,
		nil,
		nil,
		nil,
	},
	// Test errors
	{
		`NewErrors`,
		`
		contract NewErrors {
			error MyError(uint256);
			error MyError1(uint256);
			error MyError2(uint256, uint256);
			error MyError3(uint256 a, uint256 b, uint256 c);
			function Error() public pure {
				revert MyError3(1,2,3);
			}
		}
		`,
		[]string{"0x608060405234801561000f575f80fd5b506101c38061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610029575f3560e01c8063726c63821461002d575b5f80fd5b610035610037565b005b6001600260036040517f921db34000000000000000000000000000000000000000000000000000000000815260040161007293929190610138565b60405180910390fd5b5f819050919050565b5f819050919050565b5f819050919050565b5f6100b06100ab6100a68461007b565b61008d565b610084565b9050919050565b6100c081610096565b82525050565b5f819050919050565b5f6100e96100e46100df846100c6565b61008d565b610084565b9050919050565b6100f9816100cf565b82525050565b5f819050919050565b5f61012261011d610118846100ff565b61008d565b610084565b9050919050565b61013281610108565b82525050565b5f60608201905061014b5f8301866100b7565b61015860208301856100f0565b6101656040830184610129565b94935050505056fea2646970667358221220d3d03095fa0907f22220b7bad38e317f2cce027c08ccddb0a45474aebb75259964687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		[]string{`[{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"MyError","type":"error"},{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"MyError1","type":"error"},{"inputs":[{"internalType":"uint256","name":"","type":"uint256"},{"internalType":"uint256","name":"","type":"uint256"}],"name":"MyError2","type":"error"},{"inputs":[{"internalType":"uint256","name":"a","type":"uint256"},{"internalType":"uint256","name":"b","type":"uint256"},{"internalType":"uint256","name":"c","type":"uint256"}],"name":"MyError3","type":"error"},{"inputs":[],"name":"Error","outputs":[],"stateMutability":"pure","type":"function"}]`},
		`
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/qrl/qrlconfig"
		`,
		`
			var (
				wallet, _  = wallet.Generate(wallet.ML_DSA_87)
				user, _ = bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
				sim     = backends.NewSimulatedBackend(core.GenesisAlloc{user.From: {Balance: big.NewInt(1000000000000000000)}}, qrlconfig.Defaults.Miner.GasCeil)
			)
			defer sim.Close()

			_, tx, contract, err := DeployNewErrors(user, sim)
			if err != nil {
				t.Fatal(err)
			}
			sim.Commit()
			_, err = bind.WaitDeployed(nil, sim, tx)
			if err != nil {
				t.Error(err)
			}
			if err := contract.Error(new(bind.CallOpts)); err == nil {
				t.Fatalf("expected contract to throw error")
			}
			// TODO (MariusVanDerWijden unpack error using abigen
			// once that is implemented
		`,
		nil,
		nil,
		nil,
		nil,
	},
	{
		name: `ConstructorWithStructParam`,
		contract: `
		contract ConstructorWithStructParam {
			struct StructType {
				uint256 field;
			}

			constructor(StructType memory st) {}
		}
		`,
		bytecode: []string{`0x608060405234801561000f575f80fd5b506040516101d13803806101d18339818101604052810190610031919061013c565b50610167565b5f604051905090565b5f80fd5b5f80fd5b5f601f19601f8301169050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b61008e82610048565b810181811067ffffffffffffffff821117156100ad576100ac610058565b5b80604052505050565b5f6100bf610037565b90506100cb8282610085565b919050565b5f819050919050565b6100e2816100d0565b81146100ec575f80fd5b50565b5f815190506100fd816100d9565b92915050565b5f6020828403121561011857610117610044565b5b61012260206100b6565b90505f610131848285016100ef565b5f8301525092915050565b5f6020828403121561015157610150610040565b5b5f61015e84828501610103565b91505092915050565b605e806101735f395ff3fe60806040525f80fdfea2646970667358221220f8cd3f25bafb92539e9b4492292aff3e788da76cf4f4eae975d370687c9e3efa64687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053`},
		abi:      []string{`[{"inputs":[{"components":[{"internalType":"uint256","name":"field","type":"uint256"}],"internalType":"struct ConstructorWithStructParam.StructType","name":"st","type":"tuple"}],"stateMutability":"nonpayable","type":"constructor"}]`},
		imports: `
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/qrl/qrlconfig"
		`,
		tester: `
			var (
				wallet, _  = wallet.Generate(wallet.ML_DSA_87)
				user, _ = bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
				sim     = backends.NewSimulatedBackend(core.GenesisAlloc{user.From: {Balance: big.NewInt(1000000000000000000)}}, qrlconfig.Defaults.Miner.GasCeil)
			)
			defer sim.Close()

			_, tx, _, err := DeployConstructorWithStructParam(user, sim, ConstructorWithStructParamStructType{Field: big.NewInt(42)})
			if err != nil {
				t.Fatalf("DeployConstructorWithStructParam() got err %v; want nil err", err)
			}
			sim.Commit()

			if _, err = bind.WaitDeployed(nil, sim, tx); err != nil {
				t.Logf("Deployment tx: %+v", tx)
				t.Errorf("bind.WaitDeployed(nil, %T, <deployment tx>) got err %v; want nil err", sim, err)
			}
		`,
	},
	{
		name: `NameConflict`,
		contract: `
		contract oracle {
			struct request {
					bytes data;
					bytes _data;
			}
			event log (int msg, int _msg);
			function addRequest(request memory req) public pure {}
			function getRequest() pure public returns (request memory) {
					return request("", "");
			}
		}
		`,
		bytecode: []string{"0x608060405234801561000f575f80fd5b5061041f8061001d5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c8063c2bb515f14610038578063cce7b04814610056575b5f80fd5b610040610072565b60405161004d9190610198565b60405180910390f35b610070600480360381019061006b9190610382565b6100b0565b005b61007a6100b3565b604051806040016040528060405180602001604052805f815250815260200160405180602001604052805f815250815250905090565b50565b604051806040016040528060608152602001606081525090565b5f81519050919050565b5f82825260208201905092915050565b5f5b838110156101045780820151818401526020810190506100e9565b5f8484015250505050565b5f601f19601f8301169050919050565b5f610129826100cd565b61013381856100d7565b93506101438185602086016100e7565b61014c8161010f565b840191505092915050565b5f604083015f8301518482035f860152610171828261011f565b9150506020830151848203602086015261018b828261011f565b9150508091505092915050565b5f6020820190508181035f8301526101b08184610157565b905092915050565b5f604051905090565b5f80fd5b5f80fd5b5f80fd5b7f4e487b71000000000000000000000000000000000000000000000000000000005f52604160045260245ffd5b6102038261010f565b810181811067ffffffffffffffff82111715610222576102216101cd565b5b80604052505050565b5f6102346101b8565b905061024082826101fa565b919050565b5f80fd5b5f80fd5b5f80fd5b5f67ffffffffffffffff82111561026b5761026a6101cd565b5b6102748261010f565b9050602081019050919050565b828183375f83830152505050565b5f6102a161029c84610251565b61022b565b9050828152602081018484840111156102bd576102bc61024d565b5b6102c8848285610281565b509392505050565b5f82601f8301126102e4576102e3610249565b5b81356102f484826020860161028f565b91505092915050565b5f60408284031215610312576103116101c9565b5b61031c604061022b565b90505f82013567ffffffffffffffff81111561033b5761033a610245565b5b610347848285016102d0565b5f83015250602082013567ffffffffffffffff81111561036a57610369610245565b5b610376848285016102d0565b60208301525092915050565b5f60208284031215610397576103966101c1565b5b5f82013567ffffffffffffffff8111156103b4576103b36101c5565b5b6103c0848285016102fd565b9150509291505056fea26469706673582212206037744982481d94d4b04573a61ade97e7a5116a0c1ed0bad93185cb3bffa39064687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		abi:      []string{`[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"int256","name":"msg","type":"int256"},{"indexed":false,"internalType":"int256","name":"_msg","type":"int256"}],"name":"log","type":"event"},{"inputs":[{"components":[{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"bytes","name":"_data","type":"bytes"}],"internalType":"struct oracle.request","name":"req","type":"tuple"}],"name":"addRequest","outputs":[],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"getRequest","outputs":[{"components":[{"internalType":"bytes","name":"data","type":"bytes"},{"internalType":"bytes","name":"_data","type":"bytes"}],"internalType":"struct oracle.request","name":"","type":"tuple"}],"stateMutability":"pure","type":"function"}]`},
		imports: `
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/qrl/qrlconfig"
		`,
		tester: `
			var (
				wallet, _  = wallet.Generate(wallet.ML_DSA_87)
				user, _ = bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
				sim     = backends.NewSimulatedBackend(core.GenesisAlloc{user.From: {Balance: big.NewInt(1000000000000000000)}}, qrlconfig.Defaults.Miner.GasCeil)
			)
			defer sim.Close()

			_, tx, _, err := DeployNameConflict(user, sim)
			if err != nil {
				t.Fatalf("DeployNameConflict() got err %v; want nil err", err)
			}
			sim.Commit()

			if _, err = bind.WaitDeployed(nil, sim, tx); err != nil {
				t.Logf("Deployment tx: %+v", tx)
				t.Errorf("bind.WaitDeployed(nil, %T, <deployment tx>) got err %v; want nil err", sim, err)
			}
		`,
	},
	{
		name: "RangeKeyword",
		contract: `
		contract keywordcontract {
			function functionWithKeywordParameter(uint256 range) public pure {}
		}
		`,
		bytecode: []string{"0x608060405234801561000f575f80fd5b5060f38061001c5f395ff3fe6080604052348015600e575f80fd5b50600436106026575f3560e01c8063527a119f14602a575b5f80fd5b60406004803603810190603c91906077565b6042565b005b50565b5f80fd5b5f819050919050565b6059816049565b81146062575f80fd5b50565b5f813590506071816052565b92915050565b5f6020828403121560895760886045565b5b5f6094848285016065565b9150509291505056fea26469706673582212204c56b1309c07e677141a45e672c3fce67d122bfff6f3f42c1b5ef6456d3342c264687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		abi:      []string{`[{"inputs":[{"internalType":"uint256","name":"range","type":"uint256"}],"name":"functionWithKeywordParameter","outputs":[],"stateMutability":"pure","type":"function"}]`},
		imports: `
			"math/big"

			"github.com/theQRL/go-qrl/accounts/abi/bind"
			"github.com/theQRL/go-qrl/accounts/abi/bind/backends"
			"github.com/theQRL/go-qrl/core"
			"github.com/theQRL/go-qrl/crypto/pqcrypto/wallet"
			"github.com/theQRL/go-qrl/qrl/qrlconfig"
		`,
		tester: `
			var (
				wallet, _  = wallet.Generate(wallet.ML_DSA_87)
				user, _ = bind.NewKeyedTransactorWithChainID(wallet, big.NewInt(1337))
				sim     = backends.NewSimulatedBackend(core.GenesisAlloc{user.From: {Balance: big.NewInt(1000000000000000000)}}, qrlconfig.Defaults.Miner.GasCeil)
			)
			_, tx, _, err := DeployRangeKeyword(user, sim)
			if err != nil {
				t.Fatalf("error deploying contract: %v", err)
			}
			sim.Commit()

			if _, err = bind.WaitDeployed(nil, sim, tx); err != nil {
				t.Errorf("error deploying the contract: %v", err)
			}
		`,
	},
	{
		name: "NumericMethodName",
		contract: `
		contract NumericMethodName {
			event _1TestEvent(address _param);
			function _1test() public pure {}
			function __1test() public pure {}
			function __2test() public pure {}
		}
		`,
		bytecode: []string{"0x6080604052348015600e575f80fd5b5060b28061001b5f395ff3fe6080604052348015600e575f80fd5b5060043610603a575f3560e01c80639d99313214603e578063d02767c7146046578063ffa0279514604e575b5f80fd5b60446056565b005b604c6058565b005b6054605a565b005b565b565b56fea26469706673582212202d6c1deab83fc4da3da533de039b19e156b45f3a2da7ed065062a85dc76f586864687970637822302e312e302d63692e323032352e322e31372b636f6d6d69742e32333263333034320053"},
		abi:      []string{`[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"_param","type":"address"}],"name":"_1TestEvent","type":"event"},{"inputs":[],"name":"_1test","outputs":[],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"__1test","outputs":[],"stateMutability":"pure","type":"function"},{"inputs":[],"name":"__2test","outputs":[],"stateMutability":"pure","type":"function"}]`},
		imports: `
			"github.com/theQRL/go-qrl/common"
		`,
		tester: `
			if b, err := NewNumericMethodName(common.Address{}, nil); b == nil || err != nil {
				t.Fatalf("combined binding (%v) nil or error (%v) not nil", b, nil)
			}
		`,
	},
}

// Tests that packages generated by the binder can be successfully compiled and
// the requested tester run against it.
func TestGolangBindings(t *testing.T) {
	// Skip the test if no Go command can be found
	gocmd := runtime.GOROOT() + "/bin/go"
	if !common.FileExist(gocmd) {
		t.Skip("go sdk not found for testing")
	}
	// Create a temporary workspace for the test suite
	ws := t.TempDir()

	pkg := filepath.Join(ws, "bindtest")
	if err := os.MkdirAll(pkg, 0700); err != nil {
		t.Fatalf("failed to create package: %v", err)
	}
	// Generate the test suite for all the contracts
	for i, tt := range bindTests {
		t.Run(tt.name, func(t *testing.T) {
			var types []string
			if tt.types != nil {
				types = tt.types
			} else {
				types = []string{tt.name}
			}
			// Generate the binding and create a Go source file in the workspace
			bind, err := Bind(types, tt.abi, tt.bytecode, tt.fsigs, "bindtest", tt.libs, tt.aliases)
			if err != nil {
				t.Fatalf("test %d: failed to generate binding: %v", i, err)
			}
			if err = os.WriteFile(filepath.Join(pkg, strings.ToLower(tt.name)+".go"), []byte(bind), 0600); err != nil {
				t.Fatalf("test %d: failed to write binding: %v", i, err)
			}
			// Generate the test file with the injected test code
			code := fmt.Sprintf(`
			package bindtest

			import (
				"testing"
				%s
			)

			func Test%s(t *testing.T) {
				%s
			}
		`, tt.imports, tt.name, tt.tester)
			if err := os.WriteFile(filepath.Join(pkg, strings.ToLower(tt.name)+"_test.go"), []byte(code), 0600); err != nil {
				t.Fatalf("test %d: failed to write tests: %v", i, err)
			}
		})
	}
	// Convert the package to go modules and use the current source for go-ethereum
	moder := exec.Command(gocmd, "mod", "init", "bindtest")
	moder.Dir = pkg
	if out, err := moder.CombinedOutput(); err != nil {
		t.Fatalf("failed to convert binding test to modules: %v\n%s", err, out)
	}
	pwd, _ := os.Getwd()
	replacer := exec.Command(gocmd, "mod", "edit", "-x", "-require", "github.com/theQRL/go-qrl@v0.0.0", "-replace", "github.com/theQRL/go-qrl="+filepath.Join(pwd, "..", "..", "..")) // Repo root
	replacer.Dir = pkg
	if out, err := replacer.CombinedOutput(); err != nil {
		t.Fatalf("failed to replace binding test dependency to current source tree: %v\n%s", err, out)
	}
	tidier := exec.Command(gocmd, "mod", "tidy")
	tidier.Dir = pkg
	if out, err := tidier.CombinedOutput(); err != nil {
		t.Fatalf("failed to tidy Go module file: %v\n%s", err, out)
	}
	// Test the entire package and report any failures
	cmd := exec.Command(gocmd, "test", "-v", "-count", "1")
	cmd.Dir = pkg
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to run binding test: %v\n%s", err, out)
	}
}
