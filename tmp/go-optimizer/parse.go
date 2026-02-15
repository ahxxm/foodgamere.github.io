package main

// ContestResult holds the score and timing for a single contest optimization run.
type ContestResult struct {
	Name   string `json:"name"`
	RuleID int    `json:"ruleId"`
	Score  int    `json:"score"`
	TimeMs int64  `json:"timeMs"`
}

func runContest(contest *Contest, gd *GameData, cfg Config) (ContestResult, SimState) {
	opt := NewOptimizer(contest, gd, cfg)
	score, bestState, elapsed := opt.Optimize()
	return ContestResult{
		Name:   contest.Name,
		RuleID: contest.RuleID,
		Score:  score,
		TimeMs: elapsed.Milliseconds(),
	}, bestState
}

// Contest groups rules (guests) belonging to one banquet contest.
type Contest struct {
	RuleID int
	Name   string
	Rules  []Rule
}

// globalData holds intent/buff data nested under "global" in the JSON.
type globalData struct {
	Intents []Intent
	Buffs   []Intent
}

// InputData is the root structure containing all contests and global intent/buff data.
type InputData struct {
	Contests []Contest
	Global   globalData
}

// FindContest returns the contest matching the given rule ID, or nil if not found.
func FindContest(data *InputData, ruleID int) *Contest {
	for i := range data.Contests {
		if data.Contests[i].RuleID == ruleID {
			return &data.Contests[i]
		}
	}
	return nil
}

// ToGameData extracts the GameData (intents/buffs) from the input and builds
// O(1) lookup arrays indexed by IntentID / BuffID.
func (d *InputData) ToGameData() *GameData {
	gd := &GameData{
		Intents: d.Global.Intents,
		Buffs:   d.Global.Buffs,
	}

	// Build IntentByID
	maxIntentID := 0
	for i := range gd.Intents {
		if gd.Intents[i].IntentID > maxIntentID {
			maxIntentID = gd.Intents[i].IntentID
		}
	}
	if maxIntentID > 0 {
		gd.IntentByID = make([]*Intent, maxIntentID+1)
		for i := range gd.Intents {
			gd.IntentByID[gd.Intents[i].IntentID] = &gd.Intents[i]
		}
	}

	// Build BuffByID
	maxBuffID := 0
	for i := range gd.Buffs {
		if gd.Buffs[i].BuffID > maxBuffID {
			maxBuffID = gd.Buffs[i].BuffID
		}
	}
	if maxBuffID > 0 {
		gd.BuffByID = make([]*Intent, maxBuffID+1)
		for i := range gd.Buffs {
			gd.BuffByID[gd.Buffs[i].BuffID] = &gd.Buffs[i]
		}
	}

	return gd
}
