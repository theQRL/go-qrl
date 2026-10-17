#!/usr/bin/env python3
"""Rebuild the pinned migration artifact with a Hyperion compiler (not solc)."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--hypc", required=True, help="path to Hyperion hypc")
args = parser.parse_args()
directory = Path(__file__).resolve().parent
source = (directory / "LegacyQRLMigration.hyp").read_text()
request = {
    "language": "Hyperion",
    "sources": {"LegacyQRLMigration.hyp": {"content": source}},
    "settings": {
        "optimizer": {"enabled": True, "runs": 200},
        "qrvmVersion": "zond",
        "metadata": {"bytecodeHash": "none"},
        "outputSelection": {"*": {"*": ["abi", "storageLayout", "qrvm.deployedBytecode.object"]}},
    },
}
result = subprocess.run([args.hypc, "--standard-json"], input=json.dumps(request), text=True, capture_output=True, check=True)
output = json.loads(result.stdout)
errors = [error for error in output.get("errors", []) if error["severity"] == "error"]
if errors:
    raise SystemExit("\n".join(error["formattedMessage"] for error in errors))
contract = output["contracts"]["LegacyQRLMigration.hyp"]["LegacyQRLMigration"]
expected = ["snapshotHeight", "snapshotBlockHash", "snapshotCommitment", "targetChainId", "totalUnclaimed", "remaining", "accountKind", "usedOTS", "treeHeight", "threshold", "weight"]
actual = contract["storageLayout"]["storage"]
word = "t_uint512"
mapping = "t_mapping(t_uint512,t_uint512)"
nested = "t_mapping(t_uint512,t_mapping(t_uint512,t_uint512))"
expected_types = [word] * 5 + [mapping, mapping, nested, mapping, mapping, nested]
types = contract["storageLayout"]["types"]
if (
    len(actual) != len(expected)
    or any(
        entry["label"] != name
        or int(entry["slot"]) != slot
        or entry["offset"] != 0
        or entry["type"] != expected_types[slot]
        for slot, (name, entry) in enumerate(zip(expected, actual))
    )
    or types[word] != {"encoding": "inplace", "label": "uint512", "numberOfBytes": "64"}
    or any(
        types[name]["encoding"] != "mapping"
        or types[name]["key"] != word
        or types[name]["value"] != value
        or types[name]["numberOfBytes"] != "64"
        for name, value in [(mapping, word), (nested, mapping)]
    )
):
    raise SystemExit("contract storage layout changed; update and test the genesis builder first")
artifact = {
    "compiler": subprocess.check_output([args.hypc, "--version"], text=True).strip(),
    "source_sha256": hashlib.sha256(source.encode()).hexdigest(),
    "settings": request["settings"],
    "abi": contract["abi"],
    "runtime": contract["qrvm"]["deployedBytecode"]["object"],
    "storageLayout": contract["storageLayout"],
}
(directory / "LegacyQRLMigration.json").write_text(json.dumps(artifact, indent=2) + "\n")
print(f"Wrote LegacyQRLMigration.json ({len(artifact['runtime']) // 2} runtime bytes)")
