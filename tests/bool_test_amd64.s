// +build amd64

TEXT ·boolt0s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
block0:
        MOVB         x+0(FP), R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·boolt1s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R15
        XORQ         $1, R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·boolt2s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         $0, R15
        MOVB         R15, t0-1(SP)
        MOVB         x+0(FP), R15
        CMPB         R15, $0
        JEQ          block2
        JMP          block1
block1:
        MOVB         y+1(FP), R15
        MOVB         R15, t0-1(SP)
        JMP block2
block2:
        MOVB         t0-1(SP), R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·boolt3s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R15
        CMPB         R15, $0
        JEQ          block1
        MOVB         $1, R15
        MOVB         R15, t0-1(SP)
        JMP          block2
block1:
        MOVB         y+1(FP), R15
        MOVB         R15, t0-1(SP)
        JMP block2
block2:
        MOVB         t0-1(SP), R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·boolt4s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R15
        CMPB         R15, $0
        JEQ          block1
        MOVB         $1, R15
        MOVB         R15, t0-1(SP)
        JMP          block2
block1:
        MOVB         y+1(FP), R15
        MOVB         R15, t0-1(SP)
        JMP block2
block2:
        MOVB         t0-1(SP), R15
        MOVB         R15, ret0+8(FP)
        RET

TEXT ·boolt5s(SB),NOSPLIT,$8-9
        MOVB         $0, ret0+8(FP)
        MOVB         $0, t0-1(SP)
block0:
        MOVB         x+0(FP), R15
        CMPB         R15, $0
        JEQ          block1
        MOVB         $1, R15
        MOVB         R15, t0-1(SP)
        JMP          block2
block1:
        MOVB         y+1(FP), R15
        MOVB         R15, t0-1(SP)
        JMP block2
block2:
        MOVB         t0-1(SP), R15
        MOVB         R15, ret0+8(FP)
        RET
