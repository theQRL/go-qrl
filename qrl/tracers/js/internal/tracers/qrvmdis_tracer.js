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

// qrvmdisTracer returns sufficient information from a trace to perform qrvmdis-style
// disassembly.
{
	stack: [{ops: []}],

	// Sparse non-range opcodes that leave one stack result.
	singleResultOps: {0x20: true, 0x38: true, 0x3a: true, 0x3b: true, 0x3d: true, 0x3f: true, 0x51: true, 0x54: true, 0x58: true, 0x59: true, 0x5a: true, 0xf0: true, 0xf1: true, 0xf4: true, 0xf5: true, 0xfa: true},

	// Number of top stack entries to record as the previous opcode's result.
	resultCount: function(op) {
		// Arithmetic.
		if (op >= 0x01 && op <= 0x0b) {
			return 1;
		}
		// Comparison, bitwise, and shift.
		if (op >= 0x10 && op <= 0x1d) {
			return 1;
		}
		// Address and call data reads.
		if (op >= 0x30 && op <= 0x36) {
			return 1;
		}
		// Block context reads.
		if (op >= 0x40 && op <= 0x48) {
			return 1;
		}
		// PUSH0.
		if (op == 0x5f) {
			return 1;
		}
		// PUSH1..PUSH64.
		if (op >= 0x60 && op <= 0x9f) {
			return 1;
		}
		// DUP1..DUP16.
		if (op >= 0xa0 && op <= 0xaf) {
			return op - 0xa0 + 2;
		}
		// SWAP1..SWAP16.
		if (op >= 0xb0 && op <= 0xbf) {
			return op - 0xb0 + 2;
		}
		return this.singleResultOps[op] ? 1 : 0;
	},

	// result is invoked when all the opcodes have been iterated over and returns
	// the final result of the tracing.
	result: function() { return this.stack[0].ops; },

	// fault is invoked when the actual execution of an opcode fails.
	fault: function(log, db) { },

	// step is invoked for every opcode that the VM executes.
	step: function(log, db) {
		var frame = this.stack[this.stack.length - 1];

		var error = log.getError();
		if (error) {
			frame["error"] = error;
		} else if (log.getDepth() == this.stack.length) {
			opinfo = {
				op:     log.op.toNumber(),
				depth : log.getDepth(),
				result: [],
			};
			if (frame.ops.length > 0) {
				var prevop = frame.ops[frame.ops.length - 1];
				for(var i = 0; i < this.resultCount(prevop.op); i++)
					prevop.result.push(log.stack.peek(i).toString(16));
			}
			switch(log.op.toString()) {
			case "CALL":
				var instart = log.stack.peek(3).valueOf();
				var insize = log.stack.peek(4).valueOf();
				opinfo["gas"] = log.stack.peek(0).valueOf();
				opinfo["to"] = log.stack.peek(1).toString(16);
				opinfo["value"] = log.stack.peek(2).toString();
				opinfo["input"] = log.memory.slice(instart, instart + insize);
				opinfo["error"] = null;
				opinfo["return"] = null;
				opinfo["ops"] = [];
				this.stack.push(opinfo);
				break;
			case "DELEGATECALL": case "STATICCALL":
				var instart = log.stack.peek(2).valueOf();
				var insize = log.stack.peek(3).valueOf();
				opinfo["op"] =  log.op.toString();
				opinfo["gas"] =  log.stack.peek(0).valueOf();
				opinfo["to"] =  log.stack.peek(1).toString(16);
				opinfo["input"] =  log.memory.slice(instart, instart + insize);
				opinfo["error"] =  null;
				opinfo["return"] =  null;
				opinfo["ops"] = [];
				this.stack.push(opinfo);
				break;
			case "RETURN": case "REVERT":
				var out = log.stack.peek(0).valueOf();
				var outsize = log.stack.peek(1).valueOf();
				frame.return = log.memory.slice(out, out + outsize);
				break;
			case "STOP":
				frame.return = log.memory.slice(0, 0);
				break;
			case "JUMPDEST":
				opinfo["pc"] = log.getPC();
			}
			if(log.op.isPush()) {
				opinfo["len"] = log.op.toNumber() - 0x5e;
			}
			frame.ops.push(opinfo);
		} else {
			this.stack = this.stack.slice(0, log.getDepth());
		}
	}
}
