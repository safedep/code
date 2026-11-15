#include "textflag.h"

TEXT ·add(SB), NOSPLIT, $0-24
	CALL ·slow(SB)
	RET
