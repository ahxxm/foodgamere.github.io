package main

// Contest groups rules (guests) belonging to one banquet contest.
type Contest struct {
	RuleID int    `json:"ruleId"`
	Name   string `json:"name"`
	Rules  []Rule `json:"rules"`
}

// globalData holds intent/buff data nested under "global" in the JSON.
type globalData struct {
	Intents []Intent `json:"intents"`
	Buffs   []Intent `json:"buffs"`
}

// InputData is the root structure containing all contests and global intent/buff data.
type InputData struct {
	Contests []Contest  `json:"contests"`
	Global   globalData `json:"global"`
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

// ToGameData extracts the GameData (intents/buffs) from the input.
func (d *InputData) ToGameData() *GameData {
	return &GameData{
		Intents: d.Global.Intents,
		Buffs:   d.Global.Buffs,
	}
}
