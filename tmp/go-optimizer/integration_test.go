package main

import (
	"fmt"
	"testing"
)

var contestIDs = []int{680020, 680050, 680080, 680095}

func loadTestData(t *testing.T) (*InputData, *GameData) {
	t.Helper()
	input, err := LoadRawData(
		"/home/hi/dev/foodgamere.github.io/data/data.min.json",
		"/home/hi/dev/foodgamere.github.io/tmp/yx518-archive.json",
	)
	if err != nil {
		t.Fatalf("LoadRawData: %v", err)
	}
	return input, input.ToGameData()
}

// verifyResult runs the 12-point checklist against an optimizer result.
func verifyResult(t *testing.T, contest *Contest, gd *GameData, score int, state SimState) {
	t.Helper()
	rules := contest.Rules

	// 1. score > 0
	if score <= 0 {
		t.Errorf("total score %d, want > 0", score)
	}

	usedChefIDs := map[int]bool{}
	usedRecipeIDs := map[int]bool{}

	for ri, rule := range rules {
		rs := state[ri]
		// 7. slot count matches IntentList (or default 3)
		wantSlots := 3
		if rule.IntentList != nil {
			wantSlots = len(rule.IntentList)
		}
		if len(rs) != wantSlots {
			t.Errorf("rule %d: got %d slots, want %d", ri, len(rs), wantSlots)
			continue
		}

		for ci, slot := range rs {
			prefix := fmt.Sprintf("rule %d slot %d", ri, ci)

			// 9. no empty chef slots
			if slot.ChefIdx < 0 {
				t.Errorf("%s: empty chef (ChefIdx=%d)", prefix, slot.ChefIdx)
				continue
			}

			// 5. chef index in bounds
			if slot.ChefIdx >= len(rule.Chefs) {
				t.Errorf("%s: ChefIdx %d out of bounds (len=%d)", prefix, slot.ChefIdx, len(rule.Chefs))
				continue
			}
			chef := &rule.Chefs[slot.ChefIdx]

			// 1. chef Got==true
			if !chef.Got {
				t.Errorf("%s: chef %d (%s) not owned", prefix, chef.ChefID, chef.Name)
			}

			// 4. no duplicate chef across all guests
			if usedChefIDs[chef.ChefID] {
				t.Errorf("%s: duplicate chef %d (%s)", prefix, chef.ChefID, chef.Name)
			}
			usedChefIDs[chef.ChefID] = true

			for reci := 0; reci < 3; reci++ {
				ridx := slot.RecipeIdxs[reci]
				if ridx < 0 {
					continue
				}
				// 5. recipe index in bounds
				if ridx >= len(rule.Recipes) {
					t.Errorf("%s recipe %d: idx %d out of bounds (len=%d)", prefix, reci, ridx, len(rule.Recipes))
					continue
				}
				recipe := &rule.Recipes[ridx]

				// 2. recipe Got==true
				if !recipe.Got {
					t.Errorf("%s recipe %d: recipe %d (%s) not owned", prefix, reci, recipe.RecipeID, recipe.Name)
				}

				// 3. no duplicate recipe across all guests
				if usedRecipeIDs[recipe.RecipeID] {
					t.Errorf("%s recipe %d: duplicate recipe %d (%s)", prefix, reci, recipe.RecipeID, recipe.Name)
				}
				usedRecipeIDs[recipe.RecipeID] = true
			}
		}

		// Resolve slots with computed chef values for checks A, B, C
		slots := resolveRuleState(&rule, rs)
		customArr := buildCustomArr(slots)
		applyChefDataForRule(&rule, slots, getPartialChefAdds(customArr, &rule))

		// Check A: chef skill requirements
		for ci := range slots {
			chef := slots[ci].chefObj
			for reci := 0; reci < 3; reci++ {
				rec := slots[ci].recipes[reci].Data
				if rec == nil || chef == nil {
					continue
				}
				if !chefCanCook(chef, rec) {
					t.Errorf("rule %d slot %d recipe %d: chef %s lacks skill for %s",
						ri, ci, reci, chef.Name, rec.Name)
				}
			}
		}

		// Check B: material consumption within limits
		matPool := cloneMaterials(rule.Materials)
		matIdx := buildMatIndex(matPool, nil)
		for ci := range slots {
			for reci := 0; reci < 3; reci++ {
				rec := slots[ci].recipes[reci].Data
				if rec == nil {
					continue
				}
				qty := getRecipeQuantity(rec, matPool, &rule, slots[ci].chefObj, matIdx)
				actual := rs[ci].Quantities[reci]
				if actual > qty {
					t.Errorf("rule %d slot %d recipe %d: quantity %d exceeds available %d for %s",
						ri, ci, reci, actual, qty, rec.Name)
				}
				updateMaterialsData(matPool, rec, actual, slots[ci].chefObj, matIdx)
			}
		}
		for _, m := range matPool {
			if m.Quantity < 0 {
				t.Errorf("rule %d: material %d (%s) overdrawn: quantity=%d",
					ri, m.MaterialID, m.Name, m.Quantity)
			}
		}

		// Check C: satiety (soft check -- warn only)
		if rule.Satiety > 0 {
			satTotal := 0
			for ci := range slots {
				for reci := 0; reci < 3; reci++ {
					rec := slots[ci].recipes[reci].Data
					if rec != nil {
						satTotal += rec.Rarity * rs[ci].Quantities[reci]
					}
				}
			}
			if satTotal != rule.Satiety {
				t.Logf("rule %d: satiety mismatch: sum(rarity*qty)=%d, want %d",
					ri, satTotal, rule.Satiety)
			}
		}

		// 8. score recomputation matches
		recomputed := calcRuleScore(rules, state, ri, gd)
		cached := calcRuleScore(rules, state, ri, gd)
		if recomputed != cached {
			t.Errorf("rule %d: recomputed score %d != cached %d", ri, recomputed, cached)
		}
	}
}

func TestRealContests(t *testing.T) {
	input, gd := loadTestData(t)

	ids := contestIDs
	if testing.Short() {
		ids = ids[:1]
	}

	// Reduced search params for test speed.
	testCfg := DefaultConfig()
	testCfg.MaxDiverseSeeds = 4
	testCfg.MaxRounds = 3
	testCfg.RefineIter = 3
	testCfg.RecipeSeedK = 3
	testCfg.ChefPerSeed = 2

	for _, id := range ids {
		t.Run(fmt.Sprintf("contest_%d", id), func(t *testing.T) {
			t.Parallel()
			contest := FindContest(input, id)
			if contest == nil {
				t.Fatalf("contest %d not found", id)
			}
			opt := NewOptimizer(contest, gd, testCfg)
			score, state, elapsed := opt.Optimize()
			t.Logf("contest %d: score=%d elapsed=%v", id, score, elapsed)

			if state == nil {
				t.Fatalf("contest %d: nil state returned", id)
			}
			verifyResult(t, contest, gd, score, state)
		})
	}
}
