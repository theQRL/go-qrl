// Copyright 2017 The go-ethereum Authors
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

package vm

import (
	"bytes"
	"crypto/sha3"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/params"
	cryptomldsa87 "github.com/theQRL/go-qrllib/crypto/ml_dsa_87"
)

// precompiledTest defines the input/output pairs for precompiled contract tests.
type precompiledTest struct {
	Input, Expected string
	Gas             uint64
	Name            string
	NoBenchmark     bool // Benchmark primarily the worst-cases
}

// NOTE(rgeraldes24): unused at the moment
/*
// precompiledFailureTest defines the input/error pairs for precompiled
// contract failure tests.
type precompiledFailureTest struct {
	Input         string
	ExpectedError string
	Name          string
}
*/

var allPrecompiles = PrecompiledContractsZond

func precompileAddress(n string) string {
	return "Q" + strings.Repeat("0", 2*common.AddressLength-len(n)) + n
}

func testPrecompiled(addr string, test precompiledTest, t *testing.T) {
	t.Helper()
	contractAddr := common.MustParseAddress(addr)
	p := allPrecompiles[contractAddr]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in)
	t.Run(fmt.Sprintf("%s-Gas=%d", test.Name, gas), func(t *testing.T) {
		if res, _, err := RunPrecompiledContract(p, in, gas); err != nil {
			t.Error(err)
		} else if common.Bytes2Hex(res) != test.Expected {
			t.Errorf("Expected %v, got %v", test.Expected, common.Bytes2Hex(res))
		}
		if expGas := test.Gas; expGas != gas {
			t.Errorf("%v: gas wrong, expected %d, got %d", test.Name, expGas, gas)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %T modified input data", p)
		}
	})
}

func testPrecompiledOOG(addr string, test precompiledTest, t *testing.T) {
	contractAddr := common.MustParseAddress(addr)
	p := allPrecompiles[contractAddr]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in) - 1

	t.Run(fmt.Sprintf("%s-Gas=%d", test.Name, gas), func(t *testing.T) {
		_, _, err := RunPrecompiledContract(p, in, gas)
		if err.Error() != "out of gas" {
			t.Errorf("Expected error [out of gas], got [%v]", err)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}

// NOTE(rgeraldes): unused at the moment
/*
func testPrecompiledFailure(addr string, test precompiledFailureTest, t *testing.T) {
	p := allPrecompiles[common.HexToAddress(addr)]
	in := common.Hex2Bytes(test.Input)
	gas := p.RequiredGas(in)
	t.Run(test.Name, func(t *testing.T) {
		_, _, err := RunPrecompiledContract(p, in, gas)
		if err.Error() != test.ExpectedError {
			t.Errorf("Expected error [%v], got [%v]", test.ExpectedError, err)
		}
		// Verify that the precompile did not touch the input buffer
		exp := common.Hex2Bytes(test.Input)
		if !bytes.Equal(in, exp) {
			t.Errorf("Precompiled %v modified input data", addr)
		}
	})
}
*/

func benchmarkPrecompiled(addr string, test precompiledTest, bench *testing.B) {
	if test.NoBenchmark {
		return
	}
	contractAddr := common.MustParseAddress(addr)
	p := allPrecompiles[contractAddr]
	in := common.Hex2Bytes(test.Input)
	reqGas := p.RequiredGas(in)

	var (
		res  []byte
		err  error
		data = make([]byte, len(in))
	)

	bench.Run(fmt.Sprintf("%s-Gas=%d", test.Name, reqGas), func(bench *testing.B) {
		bench.ReportAllocs()
		start := time.Now()
		for bench.Loop() {
			copy(data, in)
			res, _, err = RunPrecompiledContract(p, data, reqGas)
		}
		elapsed := max(uint64(time.Since(start)), 1)
		gasUsed := reqGas * uint64(bench.N)
		bench.ReportMetric(float64(reqGas), "gas/op")
		// Keep it as uint64, multiply 100 to get two digit float later
		mgasps := (100 * 1000 * gasUsed) / elapsed
		bench.ReportMetric(float64(mgasps)/100, "mgas/s")
		//Check if it is correct
		if err != nil {
			bench.Error(err)
			return
		}
		if common.Bytes2Hex(res) != test.Expected {
			bench.Errorf("Expected %v, got %v", test.Expected, common.Bytes2Hex(res))
			return
		}
	})
}

// Benchmarks the sample inputs from the DEPOSITROOT precompile.
func BenchmarkPrecompiledDepositroot(bench *testing.B) {
	t := precompiledTest{
		Input:    "86541882a31bf9ee1d95745c0a34b8861af87b45c73a19e6b4a36f3596bf5392366c1f6d4a02fd21a4a00479346906c3136acccb28da54c543ad8f9c9b0b6551dfd58c13054a22df88ab36367e05945eda1e27ff54e666404c6b82b6976daaa655bafaf8509dbecef85d873e5984e01392faff533ca773a59afed5f7c39bda68ce65e903e90f9d7cc9a4780819fe00dc3f8dac4f98ac0ee39eeb6f504b9f233ccc9fe117c75a54f82eea1e6f7c4659cd067764a687f58abf1753e14463994da409100fe61a6ca56308c4d5d90ebc68fd59edc50f9d88ccf27c8de8ea352d56fff77ee3ccb2b78027e80e744d3b1822a265403f41dbba0cb28d913b196e0eee24d42502ec03a1d3894872b61eb14f9020f7c21af5f44e87166b71430090679f8a6f61a503ee018ec171e85bc23737a22ea51af3809d71773fede60c50403295cb8c8052d428cc3fdb67769a8dc3c324be6551a9b57b732dd7cb480894e661cf01913b7ad229918f147f94f575362e8e341d90279118cce7c5bfe46a35253da83755435204cee9b35bf04224af4daabaadc87414b011113eb46b8136e86c0192ccf28f0efcc14f9574bfcac132bbd4e0495c436e95d99af8b1aa14bdeb621aec2ce5d440834f4c1f72b5af5d17c94757eebd1ab01c106093ca97c910f685fcfb1c66dcf9a94bb3216467e45cf634b52d1067ffbc00ac65c79a31c5ff83a2dd52398327a98808533a5ccfe8d8289d775ed931e5724f6d6b698de25c0cde0917c9f05bf17cad3da83fe23c0ea5a099fa1d80d194a8fbe4d425389732dc0777799aadc9a8ccdc9affcf92f30f8a90fe46507bbd1ea232a473bad902b46934c960d03766edb4f5353fa9ff4d2111e8548141d44647b0565f53af2b751d7db63c97097f18f80d43d30288e3c9d015946a127af95ea70ca3046be81624105aee867f63c4339ac6092c42be394cd9c49a8059003c31f8d1cbba81435debafa201a3786bafbf3f3de83fd5d8e706c2b63feb11721a364144b61d08aa0cee1c764fa8fc52f08f3fbc4835f61fd7a564157f7496b52e24108f9755135fef6336bc05d824e90795f29c547e7a880eb9d32cfd82075c2e5dfea3ef8775c4a45008afa838994dd235239268c9842c8982f98870a19d3b4659ec80e5bc9bf4c5add1aa8065b0def26594426232a0d7abce7d5fad5076d783c8d8868ce7204227170d2a6cb3d51bee723d0027f708e4d3a95bb9c096eb356cf1cf8c036989f9b76a49a42280053784e4cc35c4e257b33eb16198a2fbfe422e0102be512a88559f5d602abdc4eaad074f6e15a44334e9756a803f6b6f1b85f277c14a7811909c51de6527bb3a2aa9978756290a491e9f1d5715ae61d22cae0ba2837ab0dcd6e60751d1b3be1d9433148c844301af5f370a9825f24bc529054d370a8318a0fa84e05ce3e5c37bea6761f39bb56dd997da86231598dffc089560852af76bf64170612b384b7033adfdef49659350f6b43fabcba48576f222d8f1ac1c6f22d6719d51b7e8f8bbc23f513442ded7f3a3b0ae25a08f5ed3e4a890e024731a9367da4723692a2b434fab924804a56ed9be4a0356ad3471ef867bbbfcff04cd59cd0b502afc2adaf4eb0db2fb787db357fb1f0352f7556bbe25516e41a290866e972b1c708388c3fff16810220550a69a4ca22604552214fe35baf32cada67f9b0e0fe6d422c42b810b94a216d47d1f555a286eecd8a92d10d20dbc86ec53092edb246117ffc4cd3cc1e7af0441b6f2ca90e937e052f51f54ab827bdbdba7baf7ad042e7485737817d8bf72a8e48afd6940bbbfaf8cb1995cd965285a5d1c56b60e1d4d90fe7270747884f677d0524cc3116b854e52cce9b256ef4284a0e7fb0768b3bc9a3a451bf7999da2fea500bbcfc649ec25f13fc27feee92be5a7080cd26c0749f4443d5e6cbde174291b79472ac9f609b895fa62888e6ba9facd7495c3c50b1301dcebfdfaa20ab8cf42a6626fe2bb9d7c85ea0a0386bb914be6fc8e1d45187395332a8bc543223ff6621dbc9295569422fd31d64c27c238c7f242607e81027932863da780aac3cd7c1c4372fe043207cefa880761b48ced88631a261942e40d9cc4674bb310f07943d271cd7826a96c22d274be1ec52d5a364c9182ce65c06c19c66baf97fb82a41a7425a53c255db9255b7fcc3a1905f1926625eebb88624654b6987aa5d86c17a394049ff469e6698b7d45da0bda881ad7032455eb1915d35f11a0f2f035dc782069437c7df29dc212f8a8cf3eaa3e55a58f4916859ba779454e53ebaa2b207f8ab19721db0ef4f5af6faf6569405a608aa30fbfbf6444dc759af0c61eca29d8ca29e6dd30f3cb5bd3b543efa6f0cfcffdf2bb829641e0bf09794e8c030550e4905888efcd86b68c857f78b61dd98ce95ebfe6db230504fa0a042185f276b325d248ef4037fe0f0b7789f04fc9aa8e6d4caf3d0d8043d91650fcc3de2f3a828aa881936c71eb4c707b6993eb966b36fa8f2c3be6acb9e82e2026d4a0f978786949906770321e6658edc32d5fd04f1e8f453eed40f9092686463386e1390a7f96250f967fbf3c6d90f61e2f0d67e29b0785a51cab69e09cca0e49f451adc4935e5a579f7526fd66519713a0d180e2c55834195154901140737362fd998b1424efe7b63fc8e7a916ad02cb8939d02c7959125f330182997edebab5b392747d5303f9fce769197da18afd9eb3226cfb1791213ee535cdb4214e946a6ce68bca6beef1600247d8708180e7a62c23e63dc6aa0d912da3bb08d25ac4a428cfaf597dcd44776d9e117a887b4c159a521aaca48dbe92efe88b0f9cad862b781fdc3a8a60b1a81437f8c64c317d841fd2f4893b7c3cc692f790fb88d89218aad88fb475439ab664d6840d75fc4c4e7d2efda5a77130320ac348ed328393db4a84ce217c8d83d199440bdb1703a1f64fb4d5cc707d044da884634e360fe7ba327a7eca6d319a9e71255cb00c2db1090829dee487edc347ff5c6620ab1a0105718f960edd9a35aaf6ac7bc1794fdf553485d3719ed08778ca419377e8566c27401a5b170ded2ecdf9e858d7f160654355a0b2403344ebe36eb7df0590618f13e3daa2ba23cbdca5733a0e78938ef48fd5fa9f8dad4386fc548f9b6c4461ea4b5d312b66fda72991db5e74ac273d4a848f6a5ba9944891b65b0fcef57027d95f8f9c881cb8597c4e57396734abc1f4c8dada746a7d7339ca875db3c0f5aa08b36394f3a70d7629bb5f2035f8023e793bc9e8616ecb76806bad9a223f79d49817171169e9d48b51bb2fd6550cef6b1b0562b2d76776575fb4987407773bcb6f010a21b322e639528246933e5a232ca6d1457067fa0f65af9b83dafa7358a0b4f5e8b766bd219dc3e93d424f0491e085fdc4bf53629b5bed4450df77c82d79e8cc3b25a9e559745f5d442d37f3a6e9fe169422c3cc6c35349d7373492c0c8ef177e526c359684275f7b4061afa5159f63c19e11d2db312bd48c2d9fc87fa79c3e589017a434acd196919bdb916dd8a7162c323e487b89a10451467fcfd7e1aeb12ca8968400bc1fdcf972b03520c140e74911a1f3c1c22359364acdca9b20e4bda4caf96853cda12dc571aa8e57094c85b9810f0b9eb9882dd8be03467a58250a1975fca1d84de30aca815d956c82bca26916e21621d6d054c7c5748f622b1192577beb208b7b1e4f64106cc801ed333ea7c4eb1d074d498070c98b64ca6c9274c60b95649e0a3a60080ca3961240000893c4612b107746836b9c4419f8a899d1aaa9ff50d0653170014382c9e47e345eb4d581847ffe331a56a063427079782d776a21ed28a688af080791ae48a3542d50d280c35106d33c961404d39b2161b7a0f324a49258938fb0518aa6eafe8e1f21459c091b6a7ed96d428e636eefb49371fc940b08a0f6ca3e7dd0f82180f64815f9f0dafc747b507a546e0f2f35d97c14be4742f03e57af2c1e639b3bdaffded76fb59414f58ff579eeb00b25f6634d6061c0e2a2491a4d68a50439c8a1becd6b7bf4b3496b2fe365dd1407e30f309c9c57411ce330af42b615477ba4f942e95983033df1f6bd37cec454b9ca2d5a4f4312d1115044bb93f0fc405f8400b18a099b3916450f48e6124c6c96bbebf2231da01cfeec1a57d787e6e518a6601c77a6277a603247acf66e871bb5b4eef687383a7d395b7c467bd68a4fc09fbdba9bb72d00dd80504e3a87e038014303d337cc814c1ce9df6f0345fde6ac41009869175455f102c9b18dc67b1ddb1efa9ab015e03f64cb5e4b4f81f5b59c38b0c2965dd37d3ec5811d78f93db7f117fb8ba0ebc2e8d0b01b44fcafd0a88882c2319e70977138b480177f0a231766e5ca86c9c130ca4fcc07c1162948cbbb85a18b63ec090bdda0bdba603c8a12d2f9e2e93f668fafc8399f34381512be43c566bd6da8b0bbd14ed36972e550d03e0a7fae0542dc9612b76949ba53b006631a279073b53f4a31ecbb7d370b4f9f7d8741c973fb17c07b87e0fb2bb0afe4aad6c001fc83c544368ef0bfa366434e195ca00e5c528b27a3a0772293def747f5d1cc8ec5eddbfdb09914c8cb0d92b07438835d8cd64d85f7991a54c44d5a06065008d84df617ace5f1f6c7401766e8d9a331afd6552ea2db8b35601f5a45b280c3f39a47994014ab614eabe9e37564231dab59967b4249051b5ff1eb5a9eae28e34836035201680a9118ef74b3eae9c9ab817da1f9c7c69e0b96b3cc23d8fa2cbe3bf39404658c6bdb3d33629faaf0a8ec56bfa36e8153c141f758192c0dcae1ffdd59710e281d07b722e60e51eea81116c60e1c68847981c10d7b7c3d4a693e16a1397e1f3214bac5db05a00045dac55a9eadef58f2b9848c0c8611602571cabbe98e81156cb8987d32e45335f1f5594d3c6069a418e4079864071116e8b62f39c74e5296c8b10626e1c927c695ecd66f30fe129d659e650d0685de337c66f10b2fd64c37c35a17f405bca7eac00d300991d3d7a1824054b98963da1b44e4454030933ab2b091ce63ec591feeb10d712510929f6d89a41a09f893af5ad3860d4a4e25c24c0c6e3354fb58db97deec1fc8faa884e478148b55fc2f79a5b18be5e8367e464a819fc08b06771255409e3331bee807ccac577a8903e177f7f98cbc668437d6034d3d9d77382999135a637d3d245fbc1a5d4423bb682adee03c60136cd6623aa57fa008a1aed00aeeeb9a540a80d92772a96a13d3f5f32dcaa9921fc8bedeb0dab7fa024b5837bb315d37c7a65a3191c52e30adb71d66c85f009677aefa5f097b50d8b93d6e358bce01ad48c7e7b45d1fcbccf445a8937062ba9f813e4a3797df3e093d431a55e264e4ec28922417755a66d4ab3674d575757d8008d8b6e8192690ae5772115f057cb68af1504e14eaed3f03c09d62772f74c91a23279450713de3d1eb6f9825d4ad95d0a5f0008d02089965c94185fcc52dc3c7e02736db5b0b95acb8d8549b82369bc83396cdddb663b6377770b57678f5ceca5d768bd6567f2fd15867571ad12ae78b8aeddfd57b23ee6b02ca5d7807bc4d5641641eed86d7e9b167b0c22ca024d8a95b787088a2583613cb3d920392ae7f5472e103df897a7953bba2add5f1899bfd7c49ef9a819174dd0f4c4afe552af85f04c6169b2cc3a2b94b2c8d6196cafcaff09cede59dcb87d10ce24f427c2c9c1db5a2ac8dcf04dfc2bf55d4a4d11472ce0981537230096feb821b1e0c7d6676eee1b6deb585090aa2ab4b42ecca44d5c6034be8666243a8d69b86cb7d32606151f6d5296ddfbdfeacf34626d8cbdb4823bd9b22ad4b3bdc3351d3009c6ec3c848144ea7383d5ad733d213eed846c72c05efe33f335000ad4104d04272c63640851d1ffe83e0f91a0e64fd4abbb81e04183a4fa76221329ba27202cd7fde74a718dd1dff713301b8626386ab8cd31bf461eff8921b694ffbc834bbd09b0e9206dcc0439598b83197071df28d81b1b639938eac411f2c6b976fcafc2a071c501bd9c9a44efa2e8096ae511de82c56e0ce75dd48b4d83837aec8bfe1fb6ab7e996f9eda5917677a9ef64a4fdaccd9bb360eb897296971a72c1e261ec135521af13975f119cef9fc54e5ab45931550be785ee83d3b643f0e3ec0aa786a0c4935f3b1f46bf05059cd94e5af1e35786954350a23f20f030cb4938ae521d6e49a508b57f50fd5bbee99e5b1b4487fe66abb8e5bc4a8e980afa347ffffc347c2f625ff9489df2e2e282f6bacc0f51389fe11ad0e888ce1f07c0cf10c47e7bc76f0d38c4a1340dd5aa4a811c93311a83e2409af116bec65e7b875cc51d09e3e849916d1804df7073badffbe046d88c07441c59d9fd2210267159c766e365bdd5888e2a34c3fc03d85ecc1c0bd6afc42c6aaa9b41585f6eb6c2624d9675f45c02e401367e89e0070965d313cde0ec5b47f3164d4f022c026e389c5ae2b8be7fb8b640ed5c502064c183704c914f7024995bf8144394f0a9ac51fd82fd2f0f12c2f313d87656f9d54ac3c7944c16cfcf4faf4a1f352d0b81c85dae75db2f9e73be3b2bd9e4b69696fa857946c5eb9d3d17f0f0d406d08a62d93549903926b26165a62411be5dd82fc279404a82cbe0182cefbd54912857f56c1d9588fdaa74d595d0d114dda679b716db8a3d0f89ebdac46bb77a2340c93f8b601bad16bb69ce617f46fd0cd86eee6afff7b3b17e4f719e1d8751e6a14951fc17665ec64e3b2048260d2c30535528cbe9b0d58ec071b1e6426977db06ac041a806717be0699c5100bbf5a0642d9a984c58d0b5057c077415d660af663f44d35c0a6b5dc2400d77f357e11b9b26e3def9556b0f08e2bf06a308e80c76746788608691b3af3347606cafc3489f10adace6ab5d72d249d2464e82d21f4fdf526e332fad4ed109e423474fbba1bab6fe2fcd3993242bd0db6cdcebf53e8ea4b0d46bde69ab1b91636974156df97394878925aaac96a13a0072e5452210509f271b83b324c01dea2de47a974b5a7f7a59411690aa8e19a28a23bb0ee2a2a7c8d7c49dbaf3158c4fb7534054c02b0e94f7bfd61453f039b8835f5a99a3db34f641dfb9a4c7a582efc7bc832e93b88a4e460a1a6f8c05edf82f24ec05ecfbd05bf3eb62e94516eb1208ca5e5ec5e68a24bf6d892d0e600b68564e85341c8c261ffbf624c0e7ccc8668b2b1946392a4fa941a43af792c26e24e3323694461f3cb7881a9a194d49ef22602541d5798effb060eae9c4729864b49fbbfecd10c3977dd9b16d9dba626a06af45b5abda166c54b59dbe28fe2ed8e5121c23c81093e669ecc7be894faccf7fbb1bfb5ac58e7974365b6cde092f9924f42cf4fc1fe4cd5aca4756541b11627eecd8de747efd0004fc69c34e9f2dbe7c856fcafcd74a6cc423fb83529942e6a263a37c8891722a3395db6af7e544dc4d4275fbc1167435dd0ed8ea34041e44dafdd9b26b11e602e3a7bceaa611e994c6d2ef82eb5dce44a3d859075180a2b7315272fa743161f16006b3207227a3ade54dea23289eb9ebdd8618bad8e8f0232a9ad6e81c7c7dc8c75e66ff6f7fa928053ac565e930d0923072a4d4ec67491c575e59fea9c006e65989d482caaedfc829160172dbc0f2b27fabc570771f3ef9555a410295e48d2c298a28ed359b987f4b5ef8aabf995ee2a05aa164a972daaf8bc9ae54ffb78732fd9790c5ff4af62d502b2461beba4f312ab38a8d1ac6bf1ee2fe91737d736f76c90ca75fd9086352ddaf12ffae72e5ef06f5953a3bb7dc017e868e6c0f2c355d9b4d7f6f6858d710ef3963f4f8c56e6bb0b4a929a44cf491d6713134f5aa94cf2db54e9959284e1f1476fee1fc3e2fc396c2f4fe741b11d6e99122e6e99338bdd66b7cd7e9a476fd99e7d1aacabe02bd8c8ff34fb3e3595e02b48db35f57508761c53028a5fbe958d05d0968989c4a08abf175e29ac2612a1d2700eadbf020596a8f538c06310b840008fab3239a11f6bf0b9d54db24435207dc205b88bf8e1cd28106e1c79fa5a58257579a95e9784c98f872bf80a0029ab4b502fdde4609ea975b1ac58e2bf89b061470c0366b57903a59a66e7001ba00bab99b94de5b2dae88c1a4f5365d4f032568c92ed56bf96705227a250a8fc5631816c27eae96dbfed5292c2388688a5303851da2e2e13373023ec84acd30ff0bec4e350fa788d07b15c43e99432b3ffc6ac9a440b24fe16842d928621707b1c4ed19f5c9bc408f8afd3ac2e9568455dee056d76931e497019b4510900969d4fb0d5e5902d97d6f48ab469b79bb95f73ac48a9a0d11b8d544e0cba1e80dba531b83690075e2de255d7935d13b1ad9633ff10649dc229b255097786c5dc5207db3921d211d79654dea2b684eb9e74ebd556f24b7edc678041a121c91a6577331ed39e45ec6626a2c4e87325b5ce221e8c6103bd107c48e443595379a3f79cd5404784c4a92cd42e9f671167ce6963d49dce553dedb481a275c53eb1711afb63c03cb21a2ee761de58d3cadfdfc9b5b6587996af6f8a5fee2860d0304d1550e67674a47293b4d593378bb56b24a0fee2b94b965d97906806c713f9a7e849f22f978d5f1686a0155edc0a0d7f9b83e63bf954d37a5ca5f1c7dae6c1dfa2a3bb5826e1946ffa7cd3c4cf50b11e5d06668ceba2efeffbe2f01a286e59c28f8755b7e5c40bd0b3670bb635cbeb01c1471d390262082b58c9f135cfdfee5ef1a93e0b30f62db0ffa9032e849e27f9f0ae86cafc49ce8038221d925f5ad2634ef00f0167a8f996493b848b3bbcacf96af652222814112e1a7d1284d086c77f03b9813f0d3f06bb291e4521f08ebc8b0cc0534009d84c04949116080c62b1eccff437b88e72bf7a983e0a449227e228307f4360649de1f87b4d2a025777d45972a3b9208444a5eab0edc38d7bf0524691cc1e68295f8862874016210b69c5693f7f257137906ed129059072503bf6595108cbba8f3eee56945b8056a554af97d20a1ea4cdaab0cb5e620bc005c6b148ee328e6cc27e0fc2b80bd142433d045417d379e6e29751a371df3e49f47ecaac5005c83d2594f9e25655fd1d1444a79e7cda750f5215bff6c98204b7ad66e74366097abfc31dbdd66f8c67ff001acb28f41be26e47dcb0691157d61c3f04fbb20d4d0d5b84f5c58149a88c23d02b9bc3dcfefd1a79c9987cce459827ded8fca7e0c75ade0fa2bda2d1901ba45eb8d51203e62928d3ff96c6d2248c6bd41efa324ad5e839486c5d54e05f79b9d103308003c934fe9d88a3340421fc40b4ca171915b6612ae457463d0822c3928a18becf2650b6cbe9914869c9a31a7f353e23f2e1be6d4e30d068a42350fa700903f2d4869b97f1dc03de6d0190108ff62bd6978d1668d92258cf149fa35e250e3706f9ce232b06bad3837fc5b65a073426dd7f25c25535dd5542b6cd8ea4388c2bd94dee0ded017a00fc002c755a3e7d4303023bce8fa13888ba2d69bf1639d1b6d3c0ff2d3b6072a4f4a041394b231d762691b862c1471abd2a984f1d366c1ba44f33f978de654849621c43a035e48099af0ac8e679a86f55315c612765a680c9e9d0df36f7b6d0189efefd101308106386408a27b693c534bfff4aaa9596529c1e6c6dcf898a2317d2ceb1c794c4ef88d5963d24fc293d3b03084a7c65c557c6ccbd683af121dd8e322b0cab60ee124276cbff922fede4ff6a4c2f7d4cf44cdb7c22e589dbadcffff8ab0dc234f137c43b88ca571e8413ceaf2994b208c3270befca3eb1e6d3278c9433d6b29b39a25435b92df168f1855304319f2594131a45ca4e5932db92d46c76f3437a1b9c5a4581617aa6f1fac7233d4966b006c0ed0997e8aa05f833fdbf005f61bdb6bdfa992209a2bf392ed7b9059e12de98d73f1dff909b28d5167d19d83c94c256479f3b44085e63c8d7b363f6771da9e98561916be20059676d0a80ce01e1c117b8adc795b5ce82c1dd85fcd4a4617ca950f9217aae356f3244b0ff1c1024542729d6dd405717263fa941245688dec4cb21772706dcda4c0e7311828bb81cd6f4099b74a86ecfb464d0d970bdace85aeed1b99c0edd4f3ddf411dd3ee9418d1d3f166f0d5f079b7c63ce7cb5dbafd563e7bc7bacd73dcca7eee09d978a590acc3e8dca613a1d197d6bc510a3a323bde7f242e3b9f45d44a5d8c27df2909532a41daaad61d0a371ee2763c047d811120f095aeec6e76d84874905270bbd99889beed3efe33e650cd631785b21143599726ab2ad31a29c100a1187f8b824bd783661e9e05e7c6508046e2197fa7be8612a4d0bee847374c362a8de4416f9a069e76ffe325494c1164db7da3ad45ed3f8d5d5b1124da5c8487a55f439",
		Expected: "9130e8f540b6a58a24da8b51e8661a2fbe937234490e7d59a6e2bbb99a3872e8",
		Name:     "depositroot_40000_quanta",
	}
	benchmarkPrecompiled(precompileAddress("01"), t, bench)
}

// Benchmarks the sample inputs from the SHA256 precompile.
func BenchmarkPrecompiledSha256(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "811c7003375852fabd0d362e40e68607a12bdabae61a7d068fe5fdd1dbbf2a5d",
		Name:     "128",
	}
	benchmarkPrecompiled(precompileAddress("02"), t, bench)
}

// Benchmarks the sample inputs from the identiy precompile.
func BenchmarkPrecompiledIdentity(bench *testing.B) {
	t := precompiledTest{
		Input:    "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Expected: "38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e000000000000000000000000000000000000000000000000000000000000001b38d18acb67d25c8bb9942764b62f18e17054f66a817bd4295423adf9ed98873e789d1dd423d25f0772d2748d60f7e4b81bb14d086eba8e8e8efb6dcff8a4ae02",
		Name:     "128",
	}
	benchmarkPrecompiled(precompileAddress("04"), t, bench)
}

// Tests the sample inputs from the ModExp.
func TestPrecompiledModExp(t *testing.T) {
	testJson("modexp", precompileAddress("05"), t)
}
func BenchmarkPrecompiledModExp(b *testing.B) {
	benchJson("modexp", precompileAddress("05"), b)
}

// Tests OOG
func TestPrecompiledModExpOOG(t *testing.T) {
	modexpTests, err := loadJson("modexp")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range modexpTests {
		testPrecompiledOOG(precompileAddress("05"), test, t)
	}
}

func TestPrecompiledDepositroot(t *testing.T) {
	testJson("depositroot", precompileAddress("01"), t)
}

// The depositroot input layout is consensus-relevant: it must match the
// beacon chain's DepositData SSZ container field for field, and the deposit
// contract concatenates the fields in exactly this order.
func TestDepositrootInputLayout(t *testing.T) {
	if depositRandaoCommitmentOffset != 2592+64+8 {
		t.Fatalf("randao_commitment offset %d, want %d", depositRandaoCommitmentOffset, 2592+64+8)
	}
	if depositSignatureOffset != 2592+64+8+32 {
		t.Fatalf("signature offset %d, want %d", depositSignatureOffset, 2592+64+8+32)
	}
	if depositInputLength != 7323 {
		t.Fatalf("input length %d, want 7323", depositInputLength)
	}
	// A commitment of the wrong size must be rejected by the hasher itself.
	d := &depositdata{
		PublicKey:           make([]byte, depositPublicKeyLength),
		WithdrawalRecipient: make([]byte, depositWithdrawalRecipientLength),
		RandaoCommitment:    make([]byte, 31),
		Signature:           make([]byte, depositSignatureLength),
	}
	if _, err := d.HashTreeRoot(); err == nil {
		t.Fatal("expected error for 31-byte randao commitment")
	}
}

func testJson(name, addr string, t *testing.T) {
	tests, err := loadJson(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiled(addr, test, t)
	}
}

// NOTE(rgeraldes24): unused at the moment
/*
func testJsonFail(name, addr string, t *testing.T) {
	tests, err := loadJsonFail(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiledFailure(addr, test, t)
	}
}
*/

func benchJson(name, addr string, b *testing.B) {
	tests, err := loadJson(name)
	if err != nil {
		b.Fatal(err)
	}
	for _, test := range tests {
		benchmarkPrecompiled(addr, test, b)
	}
}

// Failure tests

func loadJson(name string) ([]precompiledTest, error) {
	data, err := os.ReadFile(fmt.Sprintf("testdata/precompiles/%v.json", name))
	if err != nil {
		return nil, err
	}
	var testcases []precompiledTest
	err = json.Unmarshal(data, &testcases)
	return testcases, err
}

// NOTE(rgeraldes24): unused at the moment
/*
func loadJsonFail(name string) ([]precompiledFailureTest, error) {
	data, err := os.ReadFile(fmt.Sprintf("testdata/precompiles/fail-%v.json", name))
	if err != nil {
		return nil, err
	}
	var testcases []precompiledFailureTest
	err = json.Unmarshal(data, &testcases)
	return testcases, err
}
*/

func TestPrecompiledMLDSA87Verify(t *testing.T) {
	testJson("mldsa87_verify", precompileAddress("03"), t)
}

func TestPrecompiledMLDSA87VerifyContexts(t *testing.T) {
	want := common.LeftPadBytes([]byte{1}, WordBytes)
	tests := []struct {
		name    string
		context []byte
	}{
		{name: "empty"},
		{name: "eight bytes", context: bytes.Repeat([]byte{0x5a}, 8)},
		{name: "maximum length", context: bytes.Repeat([]byte{0xa5}, mldsa87VerifyMaxContextLength)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := newRawMLDSA87VerifyInput(t, test.context)
			output := runMLDSA87Verify(t, input)
			if !bytes.Equal(output, want) {
				t.Fatalf("verification output %x, want true", output)
			}
		})
	}
}

func TestPrecompiledMLDSA87VerifyRejectsInvalidInput(t *testing.T) {
	validInput := newRawMLDSA87VerifyInput(t, []byte("QRL"))
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "short fixed frame", mutate: func(input []byte) []byte {
			return input[:mldsa87VerifyMinInputLength-1]
		}},
		{name: "truncated context", mutate: func(input []byte) []byte {
			return input[:len(input)-1]
		}},
		{name: "trailing byte", mutate: func(input []byte) []byte {
			return append(input, 0)
		}},
		{name: "wrong context length", mutate: func(input []byte) []byte {
			input[mldsa87VerifyContextLengthOffset]--
			return input
		}},
		{name: "oversized context", mutate: func([]byte) []byte {
			input := make([]byte, mldsa87VerifyMinInputLength+mldsa87VerifyMaxContextLength+1)
			input[mldsa87VerifyContextLengthOffset] = mldsa87VerifyMaxContextLength
			return input
		}},
		{name: "wrong digest", mutate: func(input []byte) []byte {
			input[mldsa87VerifyDigestOffset] ^= 0x01
			return input
		}},
		{name: "wrong public key", mutate: func(input []byte) []byte {
			input[mldsa87VerifyPublicKeyOffset] ^= 0x01
			return input
		}},
		{name: "wrong signature", mutate: func(input []byte) []byte {
			input[mldsa87VerifySignatureOffset] ^= 0x01
			return input
		}},
		{name: "wrong context", mutate: func(input []byte) []byte {
			input[mldsa87VerifyContextOffset] ^= 0x01
			return input
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := test.mutate(common.CopyBytes(validInput))
			output := runMLDSA87Verify(t, input)
			if output != nil {
				t.Fatalf("verification output %x, want nil", output)
			}
		})
	}
}

// TestPrecompiledMLDSA87VerifyRejectsWeakKey checks that the precompile
// rejects a weak ML-DSA-87 public key: one whose t1 makes the verifier's
// reconstructed commitment w1' = UseHint(h, A*z - c*2^d*t1) independent of
// the challenge, so that (c~ = H(mu || w1Encode(0)), z = 0, h = 0) verifies
// for any digest without a secret key. The all-zero t1 is the canonical
// shape; t1 = 1023 everywhere (2^13*1023 = q - 1) and t1 = 512 everywhere
// (2^13*512 = 2^-1*(2^13 - 1) mod q, and c*(1 + x + ... + x^255) is always
// even) are two more. The key comes straight from calldata, bypassing the
// wallet constructor, so the precompile goes through
// cryptomldsa87.ParsePublicKey, which applies the weak-key rule shared by
// every QRL client; the FIPS 204 primitive deliberately does not.
func TestPrecompiledMLDSA87VerifyRejectsWeakKey(t *testing.T) {
	context := []byte("QRL")
	// The digest spans the full message representative so verification reaches
	// the public-key check after validating the frame length.
	digest := sha3.SumSHAKE256([]byte("QRL weak-key forgery test"), mldsa87VerifyDigestLength)
	tests := []struct {
		name string
		rho  byte
		t1   uint16 // every t1 coefficient
	}{
		{name: "all-zero key"},
		{name: "non-zero rho, zero t1", rho: 0xab},
		{name: "t1 all 1023", rho: 0xab, t1: 1023},
		{name: "t1 all 512", rho: 0xab, t1: 512},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var publicKey [cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES]byte
			for i := 0; i < cryptomldsa87.SEED_BYTES; i++ {
				publicKey[i] = test.rho
			}
			fillT1(&publicKey, test.t1)
			signature := forgeZeroHintSignature(publicKey, context, digest)
			input := make([]byte, 0, mldsa87VerifyMinInputLength+len(context))
			input = append(input, digest...)
			input = append(input, publicKey[:]...)
			input = append(input, signature[:]...)
			input = append(input, byte(len(context)))
			input = append(input, context...)
			if output := runMLDSA87Verify(t, input); output != nil {
				t.Fatalf("forged signature under a weak key verified: output %x, want nil", output)
			}
		})
	}
}

// fillT1 sets every t1 coefficient of a packed public key to v. t1 follows
// rho and is packed as 10-bit little-endian fields, four per five bytes.
func fillT1(publicKey *[cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES]byte, v uint16) {
	t1 := publicKey[cryptomldsa87.SEED_BYTES:]
	for i := 0; i+5 <= len(t1); i += 5 {
		packed := uint64(v) | uint64(v)<<10 | uint64(v)<<20 | uint64(v)<<30
		for j := 0; j < 5; j++ {
			t1[i+j] = byte(packed >> (8 * j))
		}
	}
}

// forgeZeroHintSignature builds the (c~, z = 0, h = 0) signature that verifies
// under a weak public key when the verifier does not reject such keys.
func forgeZeroHintSignature(publicKey [cryptomldsa87.CRYPTO_PUBLIC_KEY_BYTES]byte, context, digest []byte) [cryptomldsa87.CRYPTO_BYTES]byte {
	pre := append([]byte{0, byte(len(context))}, context...)
	tr := sha3.SumSHAKE256(publicKey[:], cryptomldsa87.TR_BYTES)
	h := sha3.NewSHAKE256()
	h.Write(tr[:])
	h.Write(pre)
	h.Write(digest)
	mu := make([]byte, cryptomldsa87.CRH_BYTES)
	h.Read(mu)

	h = sha3.NewSHAKE256()
	h.Write(mu)
	h.Write(make([]byte, cryptomldsa87.K*cryptomldsa87.POLY_W1_PACKED_BYTES)) // w1Encode(0)
	c := make([]byte, cryptomldsa87.C_TILDE_BYTES)
	h.Read(c)

	var signature [cryptomldsa87.CRYPTO_BYTES]byte
	copy(signature[:], c)
	// z = 0 packs every coefficient as GAMMA1 - 0 = 2^19, two per five bytes.
	z := signature[cryptomldsa87.C_TILDE_BYTES : cryptomldsa87.C_TILDE_BYTES+cryptomldsa87.L*cryptomldsa87.POLY_Z_PACKED_BYTES]
	for i := 0; i+5 <= len(z); i += 5 {
		z[i+2], z[i+4] = 0x08, 0x80
	}
	// The hint section stays all zero: no hints, which is a canonical encoding.
	return signature
}

func TestPrecompiledMLDSA87VerifyOOG(t *testing.T) {
	tests, err := loadJson("mldsa87_verify")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiledOOG(precompileAddress("03"), test, t)
	}
}

func BenchmarkPrecompiledMLDSA87Verify(b *testing.B) {
	benchJson("mldsa87_verify", precompileAddress("03"), b)
}

func newRawMLDSA87VerifyInput(tb testing.TB, context []byte) []byte {
	tb.Helper()
	signer, err := cryptomldsa87.New()
	if err != nil {
		tb.Fatal(err)
	}
	digest := sha3.SumSHAKE256([]byte("QRL raw ML-DSA-87 precompile test"), mldsa87VerifyDigestLength)
	signature, err := signer.Sign(context, digest)
	if err != nil {
		tb.Fatal(err)
	}
	publicKey := signer.GetPK()
	input := make([]byte, 0, mldsa87VerifyMinInputLength+len(context))
	input = append(input, digest...)
	input = append(input, publicKey[:]...)
	input = append(input, signature[:]...)
	input = append(input, byte(len(context)))
	return append(input, context...)
}

func runMLDSA87Verify(t *testing.T, input []byte) []byte {
	t.Helper()
	p := allPrecompiles[common.MustParseAddress(precompileAddress("03"))]
	output, remaining, err := RunPrecompiledContract(p, input, params.MLDSA87VerifyGas)
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("remaining gas %d, want 0", remaining)
	}
	return output
}

func TestPrecompiledXMSSVerify(t *testing.T) {
	testJson("xmss_verify", precompileAddress("07"), t)
}

func BenchmarkPrecompiledXMSSVerify(b *testing.B) {
	benchJson("xmss_verify", precompileAddress("07"), b)
}

func TestPrecompiledXMSSVerifyOOG(t *testing.T) {
	tests, err := loadJson("xmss_verify")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		testPrecompiledOOG(precompileAddress("07"), test, t)
	}
}

// xmssVerifyVector returns the message, signature and extended public key of
// the first xmss_verify test vector, a valid SHA2-256 height 4 qrllib signature.
func xmssVerifyVector(t *testing.T) (message, signature, extendedPK []byte) {
	t.Helper()
	tests, err := loadJson("xmss_verify")
	if err != nil {
		t.Fatal(err)
	}
	input := common.Hex2Bytes(tests[0].Input)
	messageEnd := xmssVerifyMessageLengthSize + int(binary.BigEndian.Uint32(input))
	keyStart := len(input) - xmssVerifyExtendedPKSize
	return input[xmssVerifyMessageLengthSize:messageEnd], input[messageEnd:keyStart], input[keyStart:]
}

func newXMSSVerifyInput(message, signature, extendedPK []byte) []byte {
	input := binary.BigEndian.AppendUint32(nil, uint32(len(message)))
	input = append(input, message...)
	input = append(input, signature...)
	return append(input, extendedPK...)
}

func runXMSSVerify(t *testing.T, input []byte) []byte {
	t.Helper()
	p := allPrecompiles[common.MustParseAddress(precompileAddress("07"))]
	output, _, err := RunPrecompiledContract(p, input, p.RequiredGas(input))
	if err != nil {
		t.Fatalf("xmssverify failed: %v", err)
	}
	return output
}

func TestPrecompiledXMSSVerifyRejectsInvalidInput(t *testing.T) {
	message, signature, extendedPK := xmssVerifyVector(t)
	if output := runXMSSVerify(t, newXMSSVerifyInput(message, signature, extendedPK)); !bytes.Equal(output, trueWord) {
		t.Fatalf("valid vector output %x, want true", output)
	}
	height := uint32(extendedPK[1]&0x0f) << 1
	flipped := func(data []byte, index int) []byte {
		data = common.CopyBytes(data)
		data[index] ^= 0x01
		return data
	}
	withDescriptor := func(b0, b1, b2 byte) []byte {
		key := common.CopyBytes(extendedPK)
		key[0], key[1], key[2] = b0, b1, b2
		return key
	}
	withIndex := func(index uint32) []byte {
		sig := common.CopyBytes(signature)
		binary.BigEndian.PutUint32(sig, index)
		return sig
	}
	tests := []struct {
		name  string
		input func() []byte
	}{
		{name: "empty input", input: func() []byte { return nil }},
		{name: "length prefix only", input: func() []byte {
			return newXMSSVerifyInput(message, signature, extendedPK)[:xmssVerifyMessageLengthSize]
		}},
		{name: "key only", input: func() []byte { return newXMSSVerifyInput(nil, nil, extendedPK) }},
		{name: "message length beyond input", input: func() []byte {
			input := newXMSSVerifyInput(message, signature, extendedPK)
			input[0] = 0xff
			return input
		}},
		{name: "message length off by one", input: func() []byte {
			input := newXMSSVerifyInput(message, signature, extendedPK)
			input[3]++
			return input
		}},
		{name: "wrong message", input: func() []byte { return newXMSSVerifyInput(flipped(message, 0), signature, extendedPK) }},
		{name: "extended message", input: func() []byte {
			return newXMSSVerifyInput(append(common.CopyBytes(message), 0), signature, extendedPK)
		}},
		{name: "wrong randomness", input: func() []byte { return newXMSSVerifyInput(message, flipped(signature, 4), extendedPK) }},
		{name: "wrong WOTS+ signature", input: func() []byte {
			return newXMSSVerifyInput(message, flipped(signature, 36+1000), extendedPK)
		}},
		{name: "wrong authentication path", input: func() []byte {
			return newXMSSVerifyInput(message, flipped(signature, len(signature)-1), extendedPK)
		}},
		{name: "other leaf index", input: func() []byte { return newXMSSVerifyInput(message, withIndex(0), extendedPK) }},
		{name: "leaf index out of range", input: func() []byte {
			return newXMSSVerifyInput(message, withIndex(1<<height), extendedPK)
		}},
		{name: "truncated signature", input: func() []byte {
			return newXMSSVerifyInput(message, signature[:len(signature)-xmssVerifyDigestSize], extendedPK)
		}},
		{name: "extended signature", input: func() []byte {
			return newXMSSVerifyInput(message, append(common.CopyBytes(signature), make([]byte, xmssVerifyDigestSize)...), extendedPK)
		}},
		{name: "misaligned signature", input: func() []byte {
			return newXMSSVerifyInput(message, signature[:len(signature)-1], extendedPK)
		}},
		{name: "wrong root", input: func() []byte { return newXMSSVerifyInput(message, signature, flipped(extendedPK, 3)) }},
		{name: "wrong public seed", input: func() []byte {
			return newXMSSVerifyInput(message, signature, flipped(extendedPK, 35))
		}},
		// The key is the trailing 67 bytes, so a shorter key borrows the end of the signature.
		{name: "key of 66 bytes", input: func() []byte { return newXMSSVerifyInput(message, signature, extendedPK[1:]) }},
		{name: "key with a leading extra byte", input: func() []byte {
			return newXMSSVerifyInput(message, signature, append([]byte{0}, extendedPK...))
		}},
		{name: "reserved descriptor byte", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(extendedPK[0], extendedPK[1], 0x01))
		}},
		{name: "address format", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(extendedPK[0], extendedPK[1]|0x10, 0))
		}},
		{name: "multi-sig signature type", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(0x10|extendedPK[0]&0x0f, extendedPK[1], 0))
		}},
		{name: "unknown signature type", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(0x20|extendedPK[0]&0x0f, extendedPK[1], 0))
		}},
		{name: "unknown hash function", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(extendedPK[0]&0xf0|0x03, extendedPK[1], 0))
		}},
		{name: "other hash function", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(extendedPK[0]^0x01, extendedPK[1], 0))
		}},
		{name: "height 2 descriptor with a matching signature", input: func() []byte {
			return newXMSSVerifyInput(message, signature[:xmssVerifySignatureBaseSize+2*xmssVerifyDigestSize], withDescriptor(extendedPK[0], extendedPK[1]&0xf0|0x01, 0))
		}},
		{name: "descriptor height mismatch", input: func() []byte {
			return newXMSSVerifyInput(message, signature, withDescriptor(extendedPK[0], extendedPK[1]&0xf0|0x03, 0))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if output := runXMSSVerify(t, test.input()); output != nil {
				t.Fatalf("verification output %x, want nil", output)
			}
		})
	}
}

func TestPrecompiledXMSSVerifyGas(t *testing.T) {
	message, signature, extendedPK := xmssVerifyVector(t)
	height := uint64(extendedPK[1]&0x0f) << 1
	hashes := func(height uint64) uint64 {
		return params.XMSSVerifyHashGas * (xmssVerifyFixedHashes + xmssVerifyHashesPerLevel*height)
	}
	messageHash := func(length int) uint64 {
		return params.Sha256BaseGas + params.Sha256PerWordGas*toWordSize(uint64(xmssVerifyMessageHashKeyLength+length))
	}
	input := newXMSSVerifyInput(message, signature, extendedPK)
	maxHeightKey := common.CopyBytes(extendedPK)
	maxHeightKey[1] = 0 // height 0 is malformed and charged as the maximum height
	oversizedLength := common.CopyBytes(input)
	oversizedLength[0] = 0xff
	tests := []struct {
		name  string
		input []byte
		want  uint64
	}{
		{name: "valid vector", input: input, want: hashes(height) + messageHash(len(message))},
		{name: "empty message", input: newXMSSVerifyInput(nil, signature, extendedPK), want: hashes(height) + messageHash(0)},
		{name: "malformed height charges the maximum", input: newXMSSVerifyInput(message, signature, maxHeightKey), want: hashes(xmssVerifyMaxHeight) + messageHash(len(message))},
		{name: "empty input", input: nil, want: hashes(xmssVerifyMaxHeight) + messageHash(0)},
		{name: "message length is capped by the input", input: oversizedLength, want: hashes(height) + messageHash(len(input)-xmssVerifyMessageLengthSize)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := (&xmssVerify{}).RequiredGas(test.input); got != test.want {
				t.Fatalf("gas %d, want %d", got, test.want)
			}
		})
	}
}
