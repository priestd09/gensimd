// +build amd64

TEXT ·adds(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        MOVL         R14, R15
        ADDL         R13, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·subs(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        MOVL         R14, R15
        SUBL         R13, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·negs(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R13
        XORQ         R14, R14
        MOVL         R14, R15
        SUBL         R13, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·muls(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        MOVL         R14, R15
        MOVL         R15, AX
        IMULL        R13
        MOVL         AX, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·divs(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        XORQ         AX, AX
        XORQ         DX, DX
        MOVL         R14, AX
        IDIVL        R13
        MOVL         AX, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·addint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        MOVB         R14, R15
        ADDB         R13, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·subint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        MOVB         R14, R15
        SUBB         R13, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·negint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R13
        XORQ         R14, R14
        MOVB         R14, R15
        SUBB         R13, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·mulint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        MOVB         R14, R15
        MOVB         R15, AX
        IMULB        R13
        MOVB         AX, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·divint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        XORQ         AX, AX
        MOVB         R14, AX
        IDIVB        R13
        MOVB         AX, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·addint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        MOVW         R14, R15
        ADDW         R13, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·subint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        MOVW         R14, R15
        SUBW         R13, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·negint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R13
        XORQ         R14, R14
        MOVW         R14, R15
        SUBW         R13, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·mulint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        MOVW         R14, R15
        MOVW         R15, AX
        IMULW        R13
        MOVW         AX, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·divint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        XORQ         AX, AX
        XORQ         DX, DX
        MOVW         R14, AX
        IDIVW        R13
        MOVW         AX, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·addint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        MOVQ         R14, R15
        ADDQ         R13, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·subint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        MOVQ         R14, R15
        SUBQ         R13, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·negint64s(SB),NOSPLIT,$16-16
        MOVQ         $0, ret0+8(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R13
        XORQ         R14, R14
        MOVQ         R14, R15
        SUBQ         R13, R15
        MOVQ         R15, ret0+8(FP)
        RET

TEXT ·mulint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        MOVQ         R14, R15
        MOVQ         R15, AX
        IMULQ        R13
        MOVQ         AX, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·divint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        XORQ         AX, AX
        XORQ         DX, DX
        MOVQ         R14, AX
        IDIVQ        R13
        MOVQ         AX, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·adduint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        MOVB         R14, R15
        ADDB         R13, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·subuint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        MOVB         R14, R15
        SUBB         R13, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·muluint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        MOVB         R14, R15
        MOVB         R15, AX
        MULB         R13
        MOVB         AX, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·divuint8s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R14
        MOVB         y+1(FP), R13
        XORQ         AX, AX
        MOVB         R14, AX
        DIVB         R13
        MOVB         AX, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·adduint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        MOVW         R14, R15
        ADDW         R13, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·subuint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        MOVW         R14, R15
        SUBW         R13, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·muluint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        MOVW         R14, R15
        MOVW         R15, AX
        MULW         R13
        MOVW         AX, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·divuint16s(SB),NOSPLIT,$8-10
        MOVW         $0, ret0+8(FP)
        MOVW         $0, t0-2(SP)
block0:
        MOVW         x+0(FP), R14
        MOVW         y+2(FP), R13
        XORQ         AX, AX
        XORQ         DX, DX
        MOVW         R14, AX
        DIVW         R13
        MOVW         AX, R15
        MOVW         R15, ret0+8(FP)
        RET

TEXT ·adduint32s(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        MOVL         R14, R15
        ADDL         R13, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·subuint32s(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        MOVL         R14, R15
        SUBL         R13, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·muluint32s(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        MOVL         R14, R15
        MOVL         R15, AX
        MULL         R13
        MOVL         AX, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·divuint32s(SB),NOSPLIT,$8-12
        MOVL         $0, ret0+8(FP)
        MOVL         $0, t0-4(SP)
block0:
        MOVL         x+0(FP), R14
        MOVL         y+4(FP), R13
        XORQ         AX, AX
        XORQ         DX, DX
        MOVL         R14, AX
        DIVL         R13
        MOVL         AX, R15
        MOVL         R15, ret0+8(FP)
        RET

TEXT ·adduint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        MOVQ         R14, R15
        ADDQ         R13, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·subuint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        MOVQ         R14, R15
        SUBQ         R13, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·muluint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        MOVQ         R14, R15
        MOVQ         R15, AX
        MULQ         R13
        MOVQ         AX, R15
        MOVQ         R15, ret0+16(FP)
        RET

TEXT ·divuint64s(SB),NOSPLIT,$16-24
        MOVQ         $0, ret0+16(FP)
        MOVQ         $0, t0-8(SP)
block0:
        MOVQ         x+0(FP), R14
        MOVQ         y+8(FP), R13
        XORQ         AX, AX
        XORQ         DX, DX
        MOVQ         R14, AX
        DIVQ         R13
        MOVQ         AX, R15
        MOVQ         R15, ret0+16(FP)
        RET
