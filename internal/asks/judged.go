package asks

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"kbase/internal/kb"
)

// JudgedHash is the hash a scored record carries of what its value was judged
// against: the digest of the ask template it is judged under, then each
// statement in order, each written behind its length so no two lists hash
// alike. The model is no part of it. It is the one computation of the hash:
// NodeJudgedHash and EdgeJudgedHash name the template an item takes.
func JudgedHash(template string, statements ...string) kb.JudgedHash {
	h := sha256.New()
	for _, s := range append([]string{template}, statements...) {
		h.Write([]byte(strconv.Itoa(len(s)) + ":" + s))
	}
	var out kb.JudgedHash
	copy(out[:], h.Sum(nil))
	return out
}

// RigorAsk is the rigor ask a node of kind is judged under: a claim's
// statement ask, an analysis's or an experiment's method ask. It is the one
// mapping from a kind to its ask, which the judged hash and a scoring run's
// call both read. A kind no rigor pass asks is an error.
func RigorAsk(kind string) (func(subject string) (string, error), error) {
	switch kind {
	case kb.NodeKindClaim:
		return ScoreRigorStatement, nil
	case kb.NodeKindAnalysis, kb.NodeKindExperiment:
		return ScoreRigorMethod, nil
	}
	return nil, fmt.Errorf("no rigor pass asks a node of kind %s", kind)
}

// NodeJudgedHash is the judged hash of a node of kind whose statement is
// statement, under the rigor ask its kind takes (RigorAsk). A kind no rigor
// pass asks is an error.
func NodeJudgedHash(kind, statement string) (kb.JudgedHash, error) {
	ask, err := RigorAsk(kind)
	if err != nil {
		return kb.JudgedHash{}, fmt.Errorf("a node of kind %s carries no judged hash: no rigor pass asks it", kind)
	}
	return judgedUnder(ask, statement)
}

// EdgeJudgedHash is the judged hash of a scoring edge whose source states
// source and whose target states target, under the bearing ask.
func EdgeJudgedHash(source, target string) (kb.JudgedHash, error) {
	return judgedUnder(ScoreBearing, source, target)
}

// judgedUnder is the judged hash of statements under the template ask renders:
// its digest is the template rendered for an empty subject, every fixed byte
// of the instruction and nothing an item varies.
func judgedUnder(ask func(string) (string, error), statements ...string) (kb.JudgedHash, error) {
	template, err := ask("")
	if err != nil {
		return kb.JudgedHash{}, err
	}
	sum := sha256.Sum256([]byte(template))
	return JudgedHash(hex.EncodeToString(sum[:]), statements...), nil
}
