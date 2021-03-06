// +build amd64 !noasm !appengine

#include "textflag.h"

TEXT ·slicet0s(SB),$24-32
        MOVQ         $0, ret0+24(FP)
block0:
        MOVQ         $0, R14
        IMUL3Q       $8, R14, R14
        MOVQ         x+0(FP), R15
        ADDQ         R14, R15
        MOVQ         (R15), R14
        MOVQ         R14, t1-16(SP)
        MOVQ         t1-16(SP), R14
        MOVQ         R14, ret0+24(FP)
        RET

TEXT ·slicet1s(SB),$24-32
        MOVQ         $0, ret0+24(FP)
block0:
        MOVQ         $1, R14
        IMUL3Q       $8, R14, R14
        MOVQ         x+0(FP), R15
        ADDQ         R14, R15
        MOVQ         (R15), R14
        MOVQ         R14, t1-16(SP)
        MOVQ         t1-16(SP), R14
        MOVQ         R14, ret0+24(FP)
        RET

TEXT ·slicet2s(SB),$72-32
        MOVQ         $0, ret0+24(FP)
block0:
        MOVQ         $0, R14
        IMUL3Q       $8, R14, R14
        MOVQ         x+0(FP), R15
        ADDQ         R14, R15
        MOVQ         (R15), R14
        MOVQ         R14, t1-16(SP)
        MOVQ         $1, R13
        IMUL3Q       $8, R13, R13
        MOVQ         x+0(FP), R14
        ADDQ         R13, R14
        MOVQ         (R14), R13
        MOVQ         R13, t3-32(SP)
        MOVQ         t1-16(SP), R12
        MOVQ         t3-32(SP), R11
        MOVQ         R12, R13
        ADDQ         R11, R13
        MOVQ         $2, R9
        IMUL3Q       $8, R9, R9
        MOVQ         x+0(FP), R10
        ADDQ         R9, R10
        MOVQ         (R10), R9
        MOVQ         R9, t6-56(SP)
        MOVQ         t6-56(SP), R8
        MOVQ         R13, R9
        ADDQ         R8, R9
        MOVQ         R9, ret0+24(FP)
        RET

