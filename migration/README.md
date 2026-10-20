# Legacy QRL mainnet redemption

This tooling exports the **last canonical block and current state** of a stopped
legacy QRL mainnet node and installs a funded redemption contract through QRVM
genesis. The contract is `LegacyQRLMigration` in `LegacyQRLMigration.hyp`.
The initial supported withdrawal flow covers native quanta held by
ordinary XMSS wallets. Support for additional assets, such as tokens, withdrawals
from Multi Sig Addresses, and withdrawals authorized by slave keys will be added
in a later release.

The repository already contains preliminary weighted multisig export and
redemption code, described below; multisig withdrawals are outside the initial
supported scope. The legacy chain must stop at this snapshot; the contract does
not synchronize later legacy transactions.

The export includes all positive native-currency wallet balances, plus the OTS
state of zero-balance multisig signatories. The legacy zero coinbase account is
the **unmined supply reserve**, not a wallet. Its balance is reported separately
and is not minted as a redeemable allocation. Token balances, outstanding
proposals and unconfirmed transactions are not native-currency holdings.

## Large snapshots and memory

The snapshot remains a single JSON file, but it is written one account at a time.
Accounts are sorted and deduplicated in a temporary LevelDB index on disk, and
the snapshot commitment is computed incrementally. The CLI also reads snapshots
one account at a time for genesis generation, signing messages and calldata.
Genesis storage entries are written incrementally, without building the complete
migration storage map in RAM. The JSON format and commitment are compatible with
the original snapshot format.

Working memory is dominated by the largest individual account, including its OTS
ranges, plus bounded database caches, database file metadata and Go runtime
overhead. The complete account collection is never held in RAM. An unusually
large single account can still require substantial memory. The supplied base
genesis and signature input files are expected to be small and are loaded into
memory.

Use `--work-dir /path/to/scratch` on any command to select the temporary index's
parent directory. Choose a filesystem backed by disk, with enough free space
for the index and database compaction, in addition to the output file. The default
is the OS temporary directory, which may be backed by RAM on some systems. The
index is removed when the command completes or returns an error. Output files
are published only after a successful write and sync; an error removes the
partial output. With `--out -`, the caller must discard stdout if the command fails.

`gqrl init` also imports incrementally, including individual storage slots within
the migration contract. It sorts accounts and slots in a temporary disk index,
builds the state trie incrementally, and writes nodes in small batches. It checks
the complete input and the existing genesis before copying state into the chain
database, and publishes the genesis block only after the state is written.
The resulting genesis hash and state root are unchanged for the same input.

For node initialization, scratch space defaults to the node's datadir. Override it
with `gqrl --datadir /path/to/node init --work-dir /path/to/scratch genesis.json`.
Allow room for a temporary copy of the input, the sorting index and derived trie,
as well as the final chain database. Contract bytecode and header fields still
need to fit in memory; account and storage-slot counts do not determine the size
of an in-memory allocation map. Both hash and path state schemes are supported.
Duplicate fields, addresses and storage slots are rejected as ambiguous input.

The database keeps the validated genesis JSON in checksummed 1 MiB chunks for
state recovery. Recovery uses the same streaming importer and requires scratch
space too. Existing databases with the old allocation record remain readable;
older binaries cannot read the new chunked recovery record. `dumpgenesis` and the
library's `ReadGenesis` API still materialize the allocation in memory.

## Export and genesis

Use a stopped, fully validated mainnet node or a consistent copy of its LevelDB
directory. The path is a required user input; the tool does not search for wallets
or automatically pick a database. It opens the existing database read-only, takes
a consistent view, checks the mainnet genesis hash, checks the canonical tip,
rejects an unfinished reorganization, and requires state version 3. It will not
rewind a current database to an earlier block. Older databases must first be
migrated using the legacy node.

```sh
go build -o /tmp/qrl-migrate ./cmd/qrl-migrate
/tmp/qrl-migrate export --state /path/to/stopped/mainnet/state \
  --work-dir /path/to/scratch --out snapshot.json
```

Snapshot balances are decimal strings. `balance_shor` is the legacy integer
balance, `quanta` is the exact decimal value, and `balance_base_units` is
the QRVM integer balance. One legacy quanta is 10^9 legacy shor and 10^18 QRVM base
units, so the integer conversion factor is **10^9**. No floating-point arithmetic
is used. Wallet balances must not exceed the node's `total_coin_supply` issuance
counter. The manifest records that counter and `supply_gap_shor` separately:
legacy mainnet can clear a banned address's balance without decrementing issuance.
Only actual positive wallet balances fund redemption; the gap is never recreated.
Review any gap against the validated legacy state when finalizing the snapshot.

Used OTS indices are exported as ordered, inclusive `used_ots_ranges`. A range
`{"first": 10, "last": 12}` means indices 10, 11 and 12 are used. The exporter
reads **every stored 8,192-index page**, checks page sizes and tree bounds, and
reconciles the bit count against each address's `used_ots_key_count` and checks
the fully-used page prefix. A missing
page cannot silently turn a recorded used index into an available one.

Choose the target chain ID in your normal base genesis file and reserve a
64-byte contract address outside the precompile set and existing allocations:

```sh
/tmp/qrl-migrate genesis --snapshot snapshot.json --genesis base-genesis.json \
  --contract "$MIGRATION_CONTRACT" --work-dir /path/to/scratch \
  --out migration-genesis.json
gqrl --datadir /path/to/new/node init --work-dir /path/to/scratch migration-genesis.json
```

The generated genesis contains the contract's **runtime bytecode**, all claim
balances, used-OTS bitmaps, multisig weights/thresholds, the snapshot commitment,
the target chain ID, and exactly the total redeemable funds. Genesis does not
execute a constructor. There is no administrator, initializer, upgrade function,
or recovery withdrawal. Existing genesis allocations are preserved; a collision
at the selected contract address is an error. Review other allocations separately
to avoid funding the same legacy holdings twice. All output files refuse to
overwrite existing files; `--out -` explicitly writes to stdout.

## Signing and redeeming

Generate the recipient with the ML-DSA-87 wallet tooling. A 64-byte address alone
does not reveal the key algorithm or prove possession of its secret key: verify
the destination in the user's wallet before signing. The contract authenticates
the exact destination through the legacy XMSS signature.

```sh
/tmp/qrl-migrate message --snapshot snapshot.json --chain-id "$CHAIN_ID" \
  --contract "$MIGRATION_CONTRACT" --source "$LEGACY_ADDRESS" \
  --recipient "$MLDSA_ADDRESS" --out signing-request.json
```

The wallet signs the **raw bytes decoded from `message`**, without applying a
second hash or a wallet-specific message prefix. They contain, in order:

```
ASCII("QRL-XMSS-MIGRATION-V1")
|| target_chain_id (64-byte unsigned big-endian)
|| migration_contract_address (64 bytes)
|| legacy_snapshot_block_hash (32 bytes)
|| snapshot_commitment (32 bytes)
|| source_legacy_address (39 bytes, without textual Q)
|| recipient_address (64 bytes)
```

Use an index unused in **both the snapshot and the wallet's local signing
history**. The wallet must durably reserve/advance that index before exposing the
signature. Neither a blockchain snapshot nor the contract can detect an index
used in an unpublished/off-chain signature. These tools never take an XMSS seed,
private key, or wallet password and never reset a wallet's index.

Save public key/signature pairs in `signatures.json`:

```json
[{"public_key": "0x<67-byte extended public key>", "signature": "0x<XMSS signature>"}]
```

For the preliminary multisig path, obtain signatures of the same message from
enough distinct original signatories to meet its original weighted threshold.
Include their pairs in that array. The tool checks the threshold and sorts them
for the contract.
Legacy zero-threshold accounts still require one original signatory's valid
signature; an empty or unrelated set of signatures cannot redeem them.

```sh
/tmp/qrl-migrate calldata --snapshot snapshot.json --chain-id "$CHAIN_ID" \
  --contract "$MIGRATION_CONTRACT" --source "$LEGACY_ADDRESS" \
  --recipient "$MLDSA_ADDRESS" --signatures signatures.json --out claim.json
```

Submit a normal signed QRVM transaction using `to`, `data`, and zero `value` from
`claim.json`; estimate its gas against the node and pay gas normally. The CLI does
not broadcast. Any funded ML-DSA-87 account or relayer may submit it. The relayer
cannot redirect the payout or deduct a fee from the entitlement: **the entire
balance goes to the signed recipient**, even if someone copies the calldata.

The on-chain entry points are `claim(bytes publicKey, bytes signature, address
recipient)` and `claimMultisig(bytes source, bytes[] publicKeys, bytes[] signatures,
address recipient)`. Ordinary claims derive the legacy address from the XMSS
extended public key; a claimant cannot supply a different balance or OTS history.
The contract calls the existing XMSS precompile at `0x07`, checks the signature's
four-byte leaf index against its own stored bitmap, and consumes that index.
Multisig and ordinary claims share the same per-signer bitmap.

Entitlements are cleared before transferring funds; transaction failure restores
the entitlement and OTS state. Every source can redeem only once. An account that
has exhausted all its OTS indices cannot redeem under the unused-OTS requirement;
there is intentionally no administrator bypass. The signature domain also binds
the source account, chain, contract and snapshot, preventing reuse for a different
claim context. Keep the destination chain ID unique.

## Rebuilding and verification

This is a **Hyperion/QRVM** contract with 64-byte VM and ABI words. An Ethereum
Solidity compiler or Ethereum ABI library is not compatible. The checked-in
artifact includes its compiler version, settings, ABI, source hash and storage
layout. Rebuild with the intended production Hyperion version and rerun tests:

```sh
python3 migration/compile.py --hypc /path/to/hypc
GOMAXPROCS=1 GOFLAGS=-p=1 go test -count=1 -parallel=1 ./migration ./cmd/qrl-migrate
```

An opt-in stress test exports and rereads a snapshot larger than 1 GiB, then
streams its genesis output to a discard sink. Run the compiled test binary to
measure the test's peak RSS separately from compiler memory use:

```sh
GOMAXPROCS=1 GOFLAGS=-p=1 go test -c -o /tmp/qrl-migration-stream.test ./migration
/usr/bin/time -v env GOMAXPROCS=1 GOMEMLIMIT=128MiB \
  QRL_MIGRATION_STRESS_ACCOUNTS=50000 /tmp/qrl-migration-stream.test \
  -test.run '^TestStreamingLargeSnapshot$' -test.v -test.parallel=1 -test.timeout=15m
```

`GOMEMLIMIT` is a soft Go runtime memory target, not a hard limit on process RSS.
The stress test needs disk space for its synthetic state, temporary index and
snapshot. Small-fixture tests also compare streaming commitments, genesis storage
and signing bytes with the in-memory path and check cleanup after failures.

The node importer has a separate opt-in stress test with one large contract and
50,000 ordinary allocations. It writes a real chain database, closes and reopens
it, then checks balances, code, storage and the archived recovery source:

```sh
GOMAXPROCS=1 GOFLAGS=-p=1 go test -c -o /tmp/gqrl-genesis-stream.test ./core
/usr/bin/time -v env GOMAXPROCS=1 GOMEMLIMIT=128MiB \
  QRL_GENESIS_STRESS_SLOTS=5500000 /tmp/gqrl-genesis-stream.test \
  -test.run '^TestStreamGenesisLarge$' -test.v -test.parallel=1 -test.timeout=30m
```

This test needs several GB of disk space. Use a disk-backed `TMPDIR` if the OS
temporary directory is backed by RAM.

A synthetic run on this VM imported a 1.04 GiB genesis with 5,500,000 storage
slots in one contract and 50,000 ordinary accounts, persisted it and reopened the
database using approximately 137 MiB peak process RSS with no swap. It took
12 minutes 26 seconds on one CPU with `GOMEMLIMIT=128MiB`. This is a measurement,
not a fixed memory ceiling; database metadata, caches and individual code or
header fields also consume memory.

The genesis builder uses the verified full-word storage layout. Mapping locations
hash two **64-byte** words, while QRVM storage keys themselves are 32 bytes.
The artifact builder checks every top-level storage slot. Integration tests load
the generated genesis JSON and execute the actual compiled contract in QRVM.

The snapshot commitment is SHA-256 of the compact Go JSON encoding of the Snapshot
structure with `commitment` set to the empty string. Exported account order is
lexicographic by canonical legacy address. It detects accidental edits; it does
not make an untrusted database authoritative. Publish the finalized snapshot and
genesis for independent verification before launching the destination chain.

Legacy schema reference: [theQRL/QRL at a98af67](https://github.com/theQRL/QRL/tree/a98af67d8d525e4ae102f77dc41c0ec0fb66db80),
specifically `qrl.proto`, `State.py`, `Block.py`, `OptimizedAddressState.py`,
`MultiSigAddressState.py`, `PaginatedBitfield.py`, and mainnet `genesis.yml`.
