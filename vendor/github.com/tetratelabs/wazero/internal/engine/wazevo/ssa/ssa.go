// Package ssa is used to construct SSA function. By nature this is free of Wasm specific thing
// and ISA.
//
// We use the "block argument" variant of SSA: https://en.wikipedia.org/wiki/Static_single-assignment_form#Block_arguments
// which is equivalent to the traditional PHI function based one, but more convenient during optimizations.
// However, in this package's source code comment, we might use PHI whenever it seems necessary in order to be aligned with
// existing literatures, e.g. SSA level optimization algorithms are often described using PHI nodes.
//
// The rationale doc for the choice of "block argument" by MLIR of LLVM is worth a read:
// https://mlir.llvm.org/docs/Rationale/Rationale/#block-arguments-vs-phi-nodes
//
// The algorithm to resolve variable definitions used here is based on the paper
// "Simple and Efficient Construction of Static Single Assignment Form": https://link.springer.com/content/pdf/10.1007/978-3-642-37051-9_6.pdf.
package ssa
