// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package riscv64

import (
	"cmd/internal/obj/riscv"
	"cmd/internal/objabi"
	"cmd/internal/sys"
	"cmd/link/internal/ld"
	"cmd/link/internal/loader"
	"cmd/link/internal/sym"
	"fmt"
	"log"
	"sync"
)

func gentext2(ctxt *ld.Link, ldr *loader.Loader) {
}

func adddynrela(target *ld.Target, syms *ld.ArchSyms, rel *sym.Symbol, s *sym.Symbol, r *sym.Reloc) {
	log.Fatalf("adddynrela not implemented")
}

func adddynrel(target *ld.Target, ldr *loader.Loader, syms *ld.ArchSyms, s *sym.Symbol, r *sym.Reloc) bool {
	log.Fatalf("adddynrel not implemented")
	return false
}

func elfreloc2(ctxt *ld.Link, ldr *loader.Loader, s loader.Sym, r loader.ExtRelocView, sectoff int64) bool {
	log.Fatalf("elfreloc2")
	return false
}

func elfsetupplt(ctxt *ld.Link, plt, gotplt *loader.SymbolBuilder, dynamic loader.Sym) {
	log.Fatalf("elfsetuplt")
}

func machoreloc1(*sys.Arch, *ld.OutBuf, *loader.Loader, loader.Sym, loader.ExtRelocView, int64) bool {
	log.Fatalf("machoreloc1 not implemented")
	return false
}

func archreloc(target *ld.Target, ldr *loader.Loader, syms *ld.ArchSyms, r loader.Reloc2, rr *loader.ExtReloc, s loader.Sym, val int64) (o int64, needExtReloc bool, ok bool) {
	rs := r.Sym()
	rs = ldr.ResolveABIAlias(rs)
	switch r.Type() {
	case objabi.R_CALLRISCV:
		// Nothing to do.
		return val, false, true

	case objabi.R_RISCV_PCREL_ITYPE, objabi.R_RISCV_PCREL_STYPE:
		pc := ldr.SymValue(s) + int64(r.Off())
		off := ldr.SymValue(rs) + r.Add() - pc

		// Generate AUIPC and second instruction immediates.
		low, high, err := riscv.Split32BitImmediate(off)
		if err != nil {
			ldr.Errorf(s, "R_RISCV_PCREL_ relocation does not fit in 32-bits: %d", off)
		}

		auipcImm, err := riscv.EncodeUImmediate(high)
		if err != nil {
			ldr.Errorf(s, "cannot encode R_RISCV_PCREL_ AUIPC relocation offset for %s: %v", ldr.SymName(rs), err)
		}

		var secondImm, secondImmMask int64
		switch r.Type() {
		case objabi.R_RISCV_PCREL_ITYPE:
			secondImmMask = riscv.ITypeImmMask
			secondImm, err = riscv.EncodeIImmediate(low)
			if err != nil {
				ldr.Errorf(s, "cannot encode R_RISCV_PCREL_ITYPE I-type instruction relocation offset for %s: %v", ldr.SymName(rs), err)
			}
		case objabi.R_RISCV_PCREL_STYPE:
			secondImmMask = riscv.STypeImmMask
			secondImm, err = riscv.EncodeSImmediate(low)
			if err != nil {
				ldr.Errorf(s, "cannot encode R_RISCV_PCREL_STYPE S-type instruction relocation offset for %s: %v", ldr.SymName(rs), err)
			}
		default:
			panic(fmt.Sprintf("Unknown relocation type: %v", r.Type()))
		}

		auipc := int64(uint32(val))
		second := int64(uint32(val >> 32))

		auipc = (auipc &^ riscv.UTypeImmMask) | int64(uint32(auipcImm))
		second = (second &^ secondImmMask) | int64(uint32(secondImm))

		return second<<32 | auipc, false, true
	}

	return val, false, false
}

func archrelocvariant(*ld.Target, *loader.Loader, loader.Reloc2, sym.RelocVariant, loader.Sym, int64) int64 {
	log.Fatalf("archrelocvariant")
	return -1
}

func asmb(ctxt *ld.Link, _ *loader.Loader) {
	if ctxt.IsELF {
		ld.Asmbelfsetup()
	}

	var wg sync.WaitGroup
	sect := ld.Segtext.Sections[0]
	offset := sect.Vaddr - ld.Segtext.Vaddr + ld.Segtext.Fileoff
	ld.WriteParallel(&wg, ld.Codeblk, ctxt, offset, sect.Vaddr, sect.Length)

	for _, sect := range ld.Segtext.Sections[1:] {
		offset := sect.Vaddr - ld.Segtext.Vaddr + ld.Segtext.Fileoff
		ld.WriteParallel(&wg, ld.Datblk, ctxt, offset, sect.Vaddr, sect.Length)
	}

	if ld.Segrodata.Filelen > 0 {
		ld.WriteParallel(&wg, ld.Datblk, ctxt, ld.Segrodata.Fileoff, ld.Segrodata.Vaddr, ld.Segrodata.Filelen)
	}

	if ld.Segrelrodata.Filelen > 0 {
		ld.WriteParallel(&wg, ld.Datblk, ctxt, ld.Segrelrodata.Fileoff, ld.Segrelrodata.Vaddr, ld.Segrelrodata.Filelen)
	}

	ld.WriteParallel(&wg, ld.Datblk, ctxt, ld.Segdata.Fileoff, ld.Segdata.Vaddr, ld.Segdata.Filelen)

	ld.WriteParallel(&wg, ld.Dwarfblk, ctxt, ld.Segdwarf.Fileoff, ld.Segdwarf.Vaddr, ld.Segdwarf.Filelen)
	wg.Wait()
}

func asmb2(ctxt *ld.Link, _ *loader.Loader) {
	ld.Symsize = 0
	ld.Lcsize = 0
	symo := uint32(0)

	if !*ld.FlagS {
		if !ctxt.IsELF {
			ld.Errorf(nil, "unsupported executable format")
		}

		symo = uint32(ld.Segdwarf.Fileoff + ld.Segdwarf.Filelen)
		symo = uint32(ld.Rnd(int64(symo), int64(*ld.FlagRound)))
		ctxt.Out.SeekSet(int64(symo))

		ld.Asmelfsym(ctxt)
		ctxt.Out.Write(ld.Elfstrdat)

		if ctxt.LinkMode == ld.LinkExternal {
			ld.Elfemitreloc(ctxt)
		}
	}

	ctxt.Out.SeekSet(0)
	switch ctxt.HeadType {
	case objabi.Hlinux:
		ld.Asmbelf(ctxt, int64(symo))
	default:
		ld.Errorf(nil, "unsupported operating system")
	}

	if *ld.FlagC {
		fmt.Printf("textsize=%d\n", ld.Segtext.Filelen)
		fmt.Printf("datsize=%d\n", ld.Segdata.Filelen)
		fmt.Printf("bsssize=%d\n", ld.Segdata.Length-ld.Segdata.Filelen)
		fmt.Printf("symsize=%d\n", ld.Symsize)
		fmt.Printf("lcsize=%d\n", ld.Lcsize)
		fmt.Printf("total=%d\n", ld.Segtext.Filelen+ld.Segdata.Length+uint64(ld.Symsize)+uint64(ld.Lcsize))
	}
}
