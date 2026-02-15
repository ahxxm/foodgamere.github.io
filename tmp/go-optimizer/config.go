package main

// Config holds search tuning parameters. Adjust these to trade speed for solution quality.
type Config struct {
	RecipeSeedK     int  // number of top recipes tried per seed position during seed generation
	ChefPerSeed     int  // number of chefs evaluated per recipe seed
	RecipeTopN      int  // how many top-ranked recipes the climbing phase considers per slot
	MaxRounds       int  // maximum hill-climbing iterations before giving up
	RefineIter      int  // maximum quick-refine passes per seed
	PreFilterTop    int  // caps the number of candidates scored precisely in recipe ranking
	MaxDiverseSeeds int  // maximum seeds kept after diversity selection
	Verbose         bool // print detailed search progress to stderr
}

// DefaultConfig returns the standard search parameters.
func DefaultConfig() Config {
	return Config{
		RecipeSeedK:     5,
		ChefPerSeed:     3,
		RecipeTopN:      5,
		MaxRounds:       5,
		RefineIter:      5,
		PreFilterTop:    50,
		MaxDiverseSeeds: 12,
	}
}
