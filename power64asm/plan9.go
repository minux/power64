// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package power64asm

import (
	"fmt"
	"strings"
)

// symNameFunc is provided to plan9Syntax that queries the symbol table for
// a given address. It returns the name and base address of the symbol
// containing the target, if any; otherwise it returns "", 0.
type symnameFunc func(uint64) (string, uint64)

// plan9Syntax returns the Go assembler syntax for the instruction.
// The syntax was originally defined by Plan 9.
// The pc is the program counter of the first instruction, used for expanding
// PC-relative addresses into absolute ones.
// The symname function queries the symbol table for the program
// being disassembled.
func Plan9Syntax(inst Inst, pc uint64, symname symnameFunc) string { // unexport
	if symname == nil {
		symname = func(uint64) (string, uint64) { return "", 0 }
	}
	if inst.Op == 0 {
		return "?"
	}
	var args []string
	for i, a := range inst.Args[:] {
		if a == nil {
			break
		}
		args = append(args, plan9Arg(&inst, i, pc, a, symname))
	}
	var op string
	op = plan9OpMap[inst.Op]
	if op == "" {
		op = strings.ToUpper(inst.Op.String())
	}
	// laid out the instruction
	switch inst.Op {
	default: // dst, sA, sB, ...
		if len(args) == 0 {
			return op
		} else if len(args) == 1 {
			return fmt.Sprintf("%s %s", op, args[0])
		}
		args = append(args, args[0])
		return op + " " + strings.Join(args[1:], ", ")
	// store instructions always have the memory operand at the end, no need to reorder
	case STB, STBU, STBX, STBUX,
		STH, STHU, STHX, STHUX,
		STW, STWU, STWX, STWUX,
		STD, STDU, STDX, STDUX,
		STQ,
		STHBRX, STWBRX:
		return op + " " + strings.Join(args, ", ")
	}
	return "?" // unreachable
}

// plan9Arg formats arg (which is the argIndex's arg in inst) according to Plan 9 rules.
// NOTE: because Plan9Syntax is the only caller of this func, and it receives a copy
//       of inst, it's ok to modify inst.Args here.
func plan9Arg(inst *Inst, argIndex int, pc uint64, arg Arg, symname symnameFunc) string {
	// special cases for load/store instructions
	if _, ok := arg.(Offset); ok {
		if argIndex+1 == len(inst.Args) || inst.Args[argIndex+1] == nil {
			panic(fmt.Errorf("wrong table: offset not followed by register"))
		}
	}
	switch arg := arg.(type) {
	case Reg:
		if isLoadStoreOp(inst.Op) && argIndex == 1 && arg == R0 {
			return "0"
		}
		if arg == R30 {
			return "g"
		}
		return strings.ToUpper(arg.String())
	case CondReg:
		if arg == CR0 && strings.HasPrefix(inst.Op.String(), "cmp") {
			return "" // don't show cr0 for cmp instructions
		} else if arg >= CR0 {
			return fmt.Sprintf("CR%d", int(arg-CR0))
		}
		bit := [4]string{"LT", "GT", "EQ", "SO"}[(arg-Cond0LT)%4]
		if arg <= Cond0SO {
			return bit
		}
		return fmt.Sprintf("4*CR%d+%s", int(arg-Cond0LT)/4, bit)
	case Imm:
		return fmt.Sprintf("%d", arg)
	case SpReg:
		return fmt.Sprintf("%d", int(arg))
	case PCRel:
		addr := pc + uint64(int64(arg))
		if s, base := symname(addr); s != "" && base == addr {
			return fmt.Sprintf("%s(SB)", s)
		} else if s != "" {
			return fmt.Sprintf("%s%+d(SB)", s, int64(addr-base))
		}
		return fmt.Sprintf("%#x", addr)
	case Label:
		return fmt.Sprintf("%#x", int(arg))
	case Offset:
		reg := inst.Args[argIndex+1].(Reg)
		removeArg(inst, argIndex+1)
		if reg == R0 {
			return fmt.Sprintf("%d(0)", int(arg))
		}
		return fmt.Sprintf("%d(R%d)", int(arg), reg-R0)
	}
	return fmt.Sprintf("???(%v)", arg)
}

var plan9OpMap = map[Op]string{
	LBZ: "MOVBZ", STB: "MOVB",
	LBZU: "MOVBZU", STBU: "MOVBU", // TODO(minux): indexed forms are not handled
	LHZ: "MOVHZ", LHA: "MOVH", STH: "MOVH",
	LHZU: "MOVHZU", STHU: "MOVHU",
	LWZ: "MOVWZ", LWA: "MOVW", STW: "MOVW",
	LWZU: "MOVWZU", STWU: "MOVWU",
	LD: "MOVD", STD: "MOVD",
	LDU: "MOVDU", STDU: "MOVDU",
	B: "BR",
}
