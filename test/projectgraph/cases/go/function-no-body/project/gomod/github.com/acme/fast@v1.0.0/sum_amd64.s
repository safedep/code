#include "textflag.h"

TEXT ·Sum(SB), NOSPLIT, $0-32
	CALL ·fallback(SB)
	RET
