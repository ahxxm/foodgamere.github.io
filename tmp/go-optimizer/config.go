package main

// Search tuning parameters. Adjust these to trade speed for solution quality.
var cfg = struct {
	// RecipeSeedK is the number of top recipes tried per seed position during seed generation.
	RecipeSeedK int
	// ChefPerSeed is the number of chefs evaluated per recipe seed.
	ChefPerSeed int
	// RecipeTopN is how many top-ranked recipes the climbing phase considers per slot.
	RecipeTopN int
	// MaxRounds is the maximum hill-climbing iterations before giving up.
	MaxRounds int
	// RefineIter is the maximum quick-refine passes per seed.
	RefineIter int
	// PreFilterTop caps the number of candidates scored precisely in recipe ranking.
	PreFilterTop int
	// MaxDiverseSeeds is the maximum seeds kept after diversity selection.
	MaxDiverseSeeds int
}{
	RecipeSeedK:     5,
	ChefPerSeed:     3,
	RecipeTopN:      5,
	MaxRounds:       5,
	RefineIter:      5,
	PreFilterTop:    50,
	MaxDiverseSeeds: 8,
}

// Verbose controls whether detailed search progress is printed to stderr.
var Verbose bool
