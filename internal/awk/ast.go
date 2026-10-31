package awk

// Indexes of the variables the interpreter treats specially.
const (
	vNF = iota
	vNR
	vFNR
	vFS
	vOFS
	vORS
	vRS
	vFILENAME
	vSUBSEP
	vCONVFMT
	vOFMT
	vRSTART
	vRLENGTH
	vARGC
	numSpecial
)

var specialNames = [numSpecial]string{
	"NF", "NR", "FNR", "FS", "OFS", "ORS", "RS", "FILENAME", "SUBSEP",
	"CONVFMT", "OFMT", "RSTART", "RLENGTH", "ARGC",
}

type expr interface{}

type (
	numLit   struct{ v Value }
	strLit   struct{ v Value }
	regexLit struct{ src string }
	varRef   struct {
		idx  int
		name string
	}
	fieldRef  struct{ index expr }
	groupExpr struct{ items []expr }
	unaryExpr struct {
		op tokKind
		e  expr
	}
	binaryExpr struct {
		op   tokKind
		l, r expr
	}
	cmpExpr struct {
		op   tokKind
		l, r expr
	}
	matchExpr struct {
		negate bool
		l, r   expr
	}
	concatExpr struct{ l, r expr }
	andExpr    struct{ l, r expr }
	orExpr     struct{ l, r expr }
	notExpr    struct{ e expr }
	condExpr   struct{ c, a, b expr }
	assignExpr struct {
		op      tokKind
		lv, rhs expr
	}
	incDecExpr struct {
		pre   bool
		delta float64
		lv    expr
	}
	callExpr struct {
		name string
		args []expr
	}
)

type stmt interface{}

type (
	exprStmt  struct{ e expr }
	printStmt struct {
		args   []expr
		printf bool
	}
	blockStmt struct{ list []stmt }
	ifStmt    struct {
		cond      expr
		then, els stmt
	}
	whileStmt struct {
		cond expr
		body stmt
	}
	doStmt struct {
		body stmt
		cond expr
	}
	forStmt struct {
		init stmt
		cond expr
		post stmt
		body stmt
	}
	nextStmt     struct{}
	nextFileStmt struct{}
	breakStmt    struct{}
	continueStmt struct{}
	exitStmt     struct{ e expr }
)

// rule is a pattern-action pair. A nil action prints the record; a nil
// pattern matches every record; r2 is the end of a range pattern.
type rule struct {
	pattern expr
	r2      expr
	ranged  bool
	action  []stmt
	inRange bool
}

// Program is a parsed AWK program.
type Program struct {
	begins [][]stmt
	ends   [][]stmt
	rules  []*rule
	vars   map[string]int
	nvars  int
}

// VarIndex returns the slot of a variable named in the program.
func (p *Program) varIndex(name string) (int, bool) {
	i, ok := p.vars[name]
	return i, ok
}
