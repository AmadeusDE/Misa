package agent

import (
	"errors"
	"math"

	"github.com/AmadeusDE/Misa/internal/ugh/llm"
	"github.com/AmadeusDE/Misa/internal/ugh/provider"
)

// ErrBudget marks a turn stopped because Config.BeforeRequest refused the
// request. It never retries and never fails over to another route.
var ErrBudget = errors.New("budget exhausted")

// BudgetError carries the refused estimate; errors.Is(err, ErrBudget).
type BudgetError struct {
	Estimate llm.Estimate
	Err      error
}

func (e *BudgetError) Error() string {
	if e.Err == nil {
		return ErrBudget.Error()
	}
	return ErrBudget.Error() + ": " + e.Err.Error()
}
func (e *BudgetError) Unwrap() error        { return e.Err }
func (e *BudgetError) Is(target error) bool { return target == ErrBudget }

// defaultMaxOutputTokens is the output size estimates assume when
// Config.MaxOutputTokens is unset; providers get no explicit max_tokens.
const defaultMaxOutputTokens = 16384

// priceOf is the model catalog's per-token price (tests replace it).
var priceOf = provider.Pricing

// estimateRequest prices an attempt of reqBytes to ref: bytes × the calibrated
// tokens per byte at the input price, plus the output cap at the output price.
func (a *Agent) estimateRequest(ref string, reqBytes int) llm.Estimate {
	ratio := a.tokPerByte
	if ratio <= 0 {
		ratio = defaultTokPerByte
	}
	est := llm.Estimate{Ref: ref, InputTokens: int(math.Ceil(float64(reqBytes) * ratio)), MaxOutputTokens: a.cfg.MaxOutputTokens}
	if est.MaxOutputTokens <= 0 {
		est.MaxOutputTokens = defaultMaxOutputTokens
	}
	if in, out, ok := priceOf(ref); ok {
		est.Cost = float64(est.InputTokens)*in + float64(est.MaxOutputTokens)*out
		est.Priced = true
	}
	return est
}

// reserveBudget asks the budget hook for the attempt. The returned release is
// never nil and must be deferred by the caller; err is a *BudgetError.
func (a *Agent) reserveBudget(reqBytes int) (func(llm.Usage), error) {
	if a.cfg.BeforeRequest == nil {
		return func(llm.Usage) {}, nil
	}
	est := a.estimateRequest(a.activeModel(), reqBytes)
	release, err := a.cfg.BeforeRequest(est)
	if err != nil {
		var be *BudgetError
		if errors.As(err, &be) {
			return nil, err
		}
		return nil, &BudgetError{Estimate: est, Err: err}
	}
	if release == nil {
		release = func(llm.Usage) {}
	}
	return release, nil
}
