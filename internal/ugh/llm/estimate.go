package llm

// Estimate is the worst-case size and price of one request attempt, handed to
// a budget hook before the request is sent.
type Estimate struct {
	Ref             string // provider/model the attempt goes to
	InputTokens     int
	MaxOutputTokens int
	// Cost is InputTokens × the input price + MaxOutputTokens × the output
	// price (USD); 0 when Priced is false.
	Cost   float64
	Priced bool // the model catalog knew this model's prices
}
