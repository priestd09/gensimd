// +build amd64 !noasm !appengine

#include "textflag.h"

TEXT ·arrayt0s(SB),$32-16
        MOVQ         $0, ret0+8(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R15
        MOVQ         R15, R14
        MOVQ         R14, t0-8(SP)
        MOVQ         $0, R13
        IMUL3Q       $8, R13, R13
        LEAQ         t0-8(SP), R14
        ADDQ         R13, R14
        MOVQ         (R14), R13
        MOVQ         R13, t2-24(SP)
        MOVQ         t2-24(SP), R13
        MOVQ         R13, ret0+8(FP)
        RET

TEXT ·arrayt1s(SB),$40-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-16(SP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R15
        MOVQ         R15, R14
        MOVQ         x+8(FP), R13
        MOVQ         R13, R12
        MOVQ         R14, t0-16(SP)
        MOVQ         R12, t0-8(SP)
        MOVQ         $1, R12
        IMUL3Q       $8, R12, R12
        LEAQ         t0-16(SP), R14
        ADDQ         R12, R14
        MOVQ         (R14), R12
        MOVQ         R12, t2-32(SP)
        MOVQ         t2-32(SP), R12
        MOVQ         R12, ret0+16(FP)
        RET

TEXT ·arrayt2s(SB),$96-32
        MOVQ         $0, ret0+24(FP)
        MOVQ         $0, t0-24(SP)
        MOVQ         $0, t0-16(SP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R15
        MOVQ         R15, R14
        MOVQ         x+8(FP), R13
        MOVQ         R13, R12
        MOVQ         x+16(FP), R11
        MOVQ         R11, R10
        MOVQ         R14, t0-24(SP)
        MOVQ         R12, t0-16(SP)
        MOVQ         R10, t0-8(SP)
        MOVQ         $0, R12
        IMUL3Q       $8, R12, R12
        LEAQ         t0-24(SP), R14
        ADDQ         R12, R14
        MOVQ         (R14), R12
        MOVQ         R12, t2-40(SP)
        MOVQ         $1, R10
        IMUL3Q       $8, R10, R10
        LEAQ         t0-24(SP), R12
        ADDQ         R10, R12
        MOVQ         (R12), R10
        MOVQ         R10, t4-56(SP)
        MOVQ         t2-40(SP), R9
        MOVQ         t4-56(SP), R8
        MOVQ         R9, R10
        ADDQ         R8, R10
        MOVQ         $2, BX
        IMUL3Q       $8, BX, BX
        LEAQ         t0-24(SP), BP
        ADDQ         BX, BP
        MOVQ         (BP), BX
        MOVQ         BX, t7-80(SP)
        MOVQ         t7-80(SP), DI
        MOVQ         R10, BX
        ADDQ         DI, BX
        MOVQ         BX, ret0+24(FP)
        RET

