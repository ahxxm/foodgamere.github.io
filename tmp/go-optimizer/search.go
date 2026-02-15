package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"
)

// ── Optimizer ───────────────────────────────────────────────────────

// Optimizer runs a multi-phase search to find the best chef-recipe assignment for a contest.
type Optimizer struct {
	rules    []Rule
	gameData *GameData

	// per-rule filtered recipe indices (equivalent to JS _menusByRule)
	menusByRule [][]int // menusByRule[ri] = list of recipe indices into rules[ri].Recipes

	// working sim state
	simState       SimState
	dirtyRules     []bool
	ruleScoreCache []int
	bestScore      int
	bestSimState   SimState
}

// NewOptimizer creates an optimizer for the given contest and game data.
func NewOptimizer(contest *Contest, gameData *GameData) *Optimizer {
	o := &Optimizer{
		rules:    contest.Rules,
		gameData: gameData,
	}
	o.buildMenus()
	return o
}

func (o *Optimizer) buildMenus() {
	o.menusByRule = make([][]int, len(o.rules))
	for ri := range o.rules {
		rule := &o.rules[ri]
		var idxs []int
		for i := range rule.Recipes {
			if rule.Recipes[i].Got {
				idxs = append(idxs, i)
			}
		}
		o.menusByRule[ri] = idxs
	}
}

// ── SimState management ─────────────────────────────────────────────

func (o *Optimizer) numChefs(ri int) int {
	if il := o.rules[ri].IntentList; il != nil {
		return len(il)
	}
	return 3
}

func (o *Optimizer) initSimState() {
	n := len(o.rules)
	o.simState = make(SimState, n)
	o.dirtyRules = make([]bool, n)
	o.ruleScoreCache = make([]int, n)
	for ri := range o.rules {
		nc := o.numChefs(ri)
		rs := make(RuleState, nc)
		for ci := 0; ci < nc; ci++ {
			rs[ci] = SlotState{ChefIdx: -1, RecipeIdxs: [3]int{-1, -1, -1}}
		}
		o.simState[ri] = rs
		o.dirtyRules[ri] = true
	}
}

func cloneSimState(s SimState) SimState {
	if s == nil {
		return nil
	}
	c := make(SimState, len(s))
	for ri, rs := range s {
		cr := make(RuleState, len(rs))
		copy(cr, rs) // SlotState is a value type with fixed arrays
		c[ri] = cr
	}
	return c
}

func (o *Optimizer) setSimState(s SimState) {
	o.simState = s
	for i := range o.dirtyRules {
		o.dirtyRules[i] = true
	}
}

func (o *Optimizer) markDirty(ri int) {
	o.dirtyRules[ri] = true
}

// ── Chef / Recipe assignment ────────────────────────────────────────

func (o *Optimizer) simSetChef(ri, ci, chefIdx int) {
	o.markDirty(ri)
	o.simState[ri][ci].ChefIdx = chefIdx
}

func (o *Optimizer) simSetRecipe(ri, ci, recSlot, recIdx int) {
	o.markDirty(ri)
	slot := &o.simState[ri][ci]
	if recIdx < 0 {
		slot.RecipeIdxs[recSlot] = -1
		slot.Quantities[recSlot] = 0
		slot.MaxQty[recSlot] = 0
		return
	}
	rule := &o.rules[ri]
	recipe := &rule.Recipes[recIdx]
	var chef *Chef
	if slot.ChefIdx >= 0 {
		chef = &rule.Chefs[slot.ChefIdx]
	}
	qty := GetRecipeQuantity(recipe, rule.Materials, rule, chef)
	if rule.DisableMultiCookbook && qty > 1 {
		qty = 1
	}
	slot.RecipeIdxs[recSlot] = recIdx
	slot.Quantities[recSlot] = qty
	slot.MaxQty[recSlot] = qty
}

func (o *Optimizer) clearSlot(ri, ci int) {
	o.markDirty(ri)
	o.simState[ri][ci] = SlotState{ChefIdx: -1, RecipeIdxs: [3]int{-1, -1, -1}}
}

// ── Score computation ───────────────────────────────────────────────

func (o *Optimizer) fastCalcScore() int {
	total := 0
	for ri := range o.rules {
		if o.dirtyRules[ri] {
			o.ruleScoreCache[ri] = CalcRuleScore(o.rules, o.simState, ri, o.gameData)
			o.dirtyRules[ri] = false
		}
		total += o.ruleScoreCache[ri]
	}
	return total
}

func (o *Optimizer) fastCalcRuleScore(ri int) int {
	if o.dirtyRules[ri] {
		o.ruleScoreCache[ri] = CalcRuleScore(o.rules, o.simState, ri, o.gameData)
		o.dirtyRules[ri] = false
	}
	return o.ruleScoreCache[ri]
}

// ── Used-set helpers ────────────────────────────────────────────────

// getUsedChefIdxs returns set of chef indices used across all rules,
// excluding the slot at (excludeRI, excludeCI).
func (o *Optimizer) getUsedChefIdxs(excludeRI, excludeCI int) map[int]bool {
	used := make(map[int]bool)
	for ri, rs := range o.simState {
		for ci, slot := range rs {
			if ri == excludeRI && ci == excludeCI {
				continue
			}
			if slot.ChefIdx >= 0 {
				// map chefId to avoid cross-rule conflicts
				used[o.rules[ri].Chefs[slot.ChefIdx].ChefID] = true
			}
		}
	}
	return used
}

// getUsedRecipeIdxs returns set of recipe indices (in rule ri) that are already
// used GLOBALLY across all rules, excluding position (excludeCI, excludeRec) in rule ri.
// Matches JS behavior: recipes are unique across ALL guests, not just within one.
func (o *Optimizer) getUsedRecipeIdxs(ri, excludeCI, excludeRec int) map[int]bool {
	// collect all used recipeIDs globally
	usedIDs := make(map[int]bool)
	for rri, rs := range o.simState {
		for ci, slot := range rs {
			for reci, idx := range slot.RecipeIdxs {
				if rri == ri && ci == excludeCI && reci == excludeRec {
					continue
				}
				if idx >= 0 {
					usedIDs[o.rules[rri].Recipes[idx].RecipeID] = true
				}
			}
		}
	}
	// map global recipeIDs back to local indices in rule ri
	used := make(map[int]bool)
	for i, r := range o.rules[ri].Recipes {
		if usedIDs[r.RecipeID] {
			used[i] = true
		}
	}
	return used
}

// ── Skill check ─────────────────────────────────────────────────────

func chefCanCook(chef *Chef, r *Recipe) bool {
	if chef == nil || r == nil {
		return true
	}
	if r.Stirfry > 0 && chef.StirfryVal < float64(r.Stirfry) {
		return false
	}
	if r.Boil > 0 && chef.BoilVal < float64(r.Boil) {
		return false
	}
	if r.Knife > 0 && chef.KnifeVal < float64(r.Knife) {
		return false
	}
	if r.Fry > 0 && chef.FryVal < float64(r.Fry) {
		return false
	}
	if r.Bake > 0 && chef.BakeVal < float64(r.Bake) {
		return false
	}
	if r.Steam > 0 && chef.SteamVal < float64(r.Steam) {
		return false
	}
	return true
}

// ── Recipe ranking ──────────────────────────────────────────────────

type recipeRank struct {
	recIdx int
	score  int
}

func (o *Optimizer) getRecipeRanking(ri, ci, recSlot, topK int, fastMode bool) []recipeRank {
	rule := &o.rules[ri]
	menus := o.menusByRule[ri]
	usedIdxs := o.getUsedRecipeIdxs(ri, ci, recSlot)

	slot := &o.simState[ri][ci]
	var chef *Chef
	if slot.ChefIdx >= 0 {
		chef = &rule.Chefs[slot.ChefIdx]
	}

	// save current recipe at this slot
	savedIdx := slot.RecipeIdxs[recSlot]
	savedQty := slot.Quantities[recSlot]
	savedMax := slot.MaxQty[recSlot]

	// Phase 1: rough estimate (price * qty)
	type rough struct {
		recIdx int
		est    int
	}
	phase1 := make([]rough, 0, len(menus))
	for _, recIdx := range menus {
		if usedIdxs[recIdx] {
			continue
		}
		recipe := &rule.Recipes[recIdx]
		if !chefCanCook(chef, recipe) {
			continue
		}
		qty := GetRecipeQuantity(recipe, rule.Materials, rule, chef)
		if rule.DisableMultiCookbook && qty > 1 {
			qty = 1
		}
		est := int(recipe.Price * float64(qty))
		phase1 = append(phase1, rough{recIdx, est})
	}
	sort.Slice(phase1, func(i, j int) bool { return phase1[i].est > phase1[j].est })

	// Phase 3: precise scoring on top candidates (no artificial cap)
	limit := len(phase1)
	if cap := intMax(topK*2, cfg.PreFilterTop); limit > cap {
		limit = cap
	}

	results := make([]recipeRank, 0, limit)
	for i := 0; i < limit; i++ {
		o.simSetRecipe(ri, ci, recSlot, phase1[i].recIdx)
		var score int
		if fastMode {
			score = o.fastCalcRuleScore(ri)
		} else {
			score = o.fastCalcScore()
		}
		results = append(results, recipeRank{phase1[i].recIdx, score})
	}

	// restore
	slot.RecipeIdxs[recSlot] = savedIdx
	slot.Quantities[recSlot] = savedQty
	slot.MaxQty[recSlot] = savedMax
	o.markDirty(ri)

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// ── Chef ranking ────────────────────────────────────────────────────

type chefRank struct {
	chefIdx int
	score   int
	skillOk bool
}

func (o *Optimizer) getChefRanking(ri, ci int, fastMode bool) []chefRank {
	rule := &o.rules[ri]
	usedIDs := o.getUsedChefIdxs(ri, ci)
	slot := &o.simState[ri][ci]

	savedChefIdx := slot.ChefIdx

	hasRecipe := slot.RecipeIdxs[0] >= 0 || slot.RecipeIdxs[1] >= 0 || slot.RecipeIdxs[2] >= 0

	results := make([]chefRank, 0, len(rule.Chefs))
	if !hasRecipe {
		for idx := range rule.Chefs {
			if !rule.Chefs[idx].Got {
				continue
			}
			results = append(results, chefRank{idx, rule.Chefs[idx].Rarity, true})
		}
	} else {
		for idx := range rule.Chefs {
			ch := &rule.Chefs[idx]
			if !ch.Got {
				continue
			}
			if fastMode && usedIDs[ch.ChefID] {
				continue
			}
			// skill check against all recipes in slot
			ok := true
			for reci := 0; reci < 3; reci++ {
				ri2 := slot.RecipeIdxs[reci]
				if ri2 >= 0 && !chefCanCook(ch, &rule.Recipes[ri2]) {
					ok = false
					break
				}
			}
			if !ok {
				results = append(results, chefRank{idx, -1, false})
				continue
			}
			o.simSetChef(ri, ci, idx)
			var score int
			if fastMode {
				score = o.fastCalcRuleScore(ri)
			} else {
				score = o.fastCalcScore()
			}
			results = append(results, chefRank{idx, score, true})
		}
		// restore
		o.simSetChef(ri, ci, savedChefIdx)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	return results
}

// ── Greedy fill ─────────────────────────────────────────────────────

func (o *Optimizer) greedyFillRecipes(ri, ci int) {
	for reci := 0; reci < 3; reci++ {
		if o.simState[ri][ci].RecipeIdxs[reci] >= 0 {
			continue
		}
		rk := o.getRecipeRanking(ri, ci, reci, 1, true)
		if len(rk) > 0 {
			o.simSetRecipe(ri, ci, reci, rk[0].recIdx)
		}
	}
}

func (o *Optimizer) greedyFillPosition(ri, ci int) {
	// 1. seed recipe at slot 0
	rk := o.getRecipeRanking(ri, ci, 0, 1, true)
	if len(rk) > 0 {
		o.simSetRecipe(ri, ci, 0, rk[0].recIdx)
	}
	// 2. best chef
	ck := o.getChefRanking(ri, ci, true)
	usedIDs := o.getUsedChefIdxs(ri, ci)
	rule := &o.rules[ri]
	for _, c := range ck {
		if !c.skillOk || usedIDs[rule.Chefs[c.chefIdx].ChefID] {
			continue
		}
		o.simSetChef(ri, ci, c.chefIdx)
		break
	}
	// 3. fill remaining recipe slots
	o.greedyFillRecipes(ri, ci)
}

func (o *Optimizer) greedyFillGuest(ri int) {
	nc := o.numChefs(ri)
	for ci := 0; ci < nc; ci++ {
		o.greedyFillPosition(ri, ci)
	}
}

// ── Quick refine ────────────────────────────────────────────────────

func (o *Optimizer) quickRefine(activeRules []int, light bool) {
	maxIter := cfg.RefineIter
	skipChef := false
	if light {
		maxIter = 1
		skipChef = true
	}

	for iter := 0; iter < maxIter; iter++ {
		changed := false
		for _, ri := range activeRules {
			rule := &o.rules[ri]
			nc := o.numChefs(ri)
			for ci := 0; ci < nc; ci++ {
				// recipe pass
				for reci := 0; reci < 3; reci++ {
					curIdx := o.simState[ri][ci].RecipeIdxs[reci]
					rk := o.getRecipeRanking(ri, ci, reci, 1, true)
					if len(rk) > 0 && rk[0].recIdx != curIdx {
						o.simSetRecipe(ri, ci, reci, rk[0].recIdx)
						changed = true
					}
				}
				if skipChef {
					continue
				}
				// chef pass
				curChefIdx := o.simState[ri][ci].ChefIdx
				ck := o.getChefRanking(ri, ci, true)
				usedIDs := o.getUsedChefIdxs(ri, ci)
				for _, c := range ck {
					if !c.skillOk || usedIDs[rule.Chefs[c.chefIdx].ChefID] {
						continue
					}
					if c.chefIdx != curChefIdx {
						o.simSetChef(ri, ci, c.chefIdx)
						changed = true
					}
					break
				}
				// recipe pass 2 (chef may have changed)
				for reci := 0; reci < 3; reci++ {
					curIdx := o.simState[ri][ci].RecipeIdxs[reci]
					rk := o.getRecipeRanking(ri, ci, reci, 1, true)
					if len(rk) > 0 && rk[0].recIdx != curIdx {
						o.simSetRecipe(ri, ci, reci, rk[0].recIdx)
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
}

// ── Climbing ────────────────────────────────────────────────────────

func (o *Optimizer) climbChefs() bool {
	improved := false
	for ri := range o.rules {
		rule := &o.rules[ri]
		nc := o.numChefs(ri)
		for ci := 0; ci < nc; ci++ {
			curChefIdx := o.simState[ri][ci].ChefIdx
			usedIDs := o.getUsedChefIdxs(ri, ci)
			currentRS := o.fastCalcRuleScore(ri)

			ck := o.getChefRanking(ri, ci, true)
			for _, c := range ck {
				if usedIDs[rule.Chefs[c.chefIdx].ChefID] || c.chefIdx == curChefIdx || !c.skillOk {
					continue
				}
				if c.score > currentRS {
					o.simSetChef(ri, ci, c.chefIdx)
					if total := o.fastCalcScore(); total > o.bestScore {
						o.bestScore = total
						o.bestSimState = cloneSimState(o.simState)
						improved = true
					} else {
						o.simSetChef(ri, ci, curChefIdx)
					}
				}
				break // sorted desc, first usable is best
			}
		}
	}
	return improved
}

func (o *Optimizer) climbChefSwap() bool {
	improved := false
	type pos struct{ ri, ci int }
	var positions []pos
	for ri, rs := range o.simState {
		for ci, slot := range rs {
			if slot.ChefIdx >= 0 {
				positions = append(positions, pos{ri, ci})
			}
		}
	}

	for i := 0; i < len(positions)-1; i++ {
		for j := i + 1; j < len(positions); j++ {
			p1, p2 := positions[i], positions[j]
			idx1 := o.simState[p1.ri][p1.ci].ChefIdx
			idx2 := o.simState[p2.ri][p2.ci].ChefIdx
			if idx1 < 0 || idx2 < 0 {
				continue
			}

			// For cross-rule swaps, chef indices are local to each rule's Chefs slice.
			// Translate via ChefID so we assign the correct chef in each rule's pool.
			swapIdx1, swapIdx2 := idx2, idx1 // same-rule: direct swap
			if p1.ri != p2.ri {
				chefID1 := o.rules[p1.ri].Chefs[idx1].ChefID
				chefID2 := o.rules[p2.ri].Chefs[idx2].ChefID
				swapIdx1 = o.findChefIdx(p1.ri, chefID2)
				swapIdx2 = o.findChefIdx(p2.ri, chefID1)
				if swapIdx1 < 0 || swapIdx2 < 0 {
					continue // chef not in the other rule's pool
				}
			}

			// swap
			o.simSetChef(p1.ri, p1.ci, swapIdx1)
			o.simSetChef(p2.ri, p2.ci, swapIdx2)
			// recalc recipe quantities after chef swap
			for reci := 0; reci < 3; reci++ {
				if ri := o.simState[p1.ri][p1.ci].RecipeIdxs[reci]; ri >= 0 {
					o.simSetRecipe(p1.ri, p1.ci, reci, ri)
				}
				if ri := o.simState[p2.ri][p2.ci].RecipeIdxs[reci]; ri >= 0 {
					o.simSetRecipe(p2.ri, p2.ci, reci, ri)
				}
			}
			if total := o.fastCalcScore(); total > o.bestScore {
				o.bestScore = total
				o.bestSimState = cloneSimState(o.simState)
				improved = true
			} else {
				// revert using original indices
				o.simSetChef(p1.ri, p1.ci, idx1)
				o.simSetChef(p2.ri, p2.ci, idx2)
				for reci := 0; reci < 3; reci++ {
					if ri := o.simState[p1.ri][p1.ci].RecipeIdxs[reci]; ri >= 0 {
						o.simSetRecipe(p1.ri, p1.ci, reci, ri)
					}
					if ri := o.simState[p2.ri][p2.ci].RecipeIdxs[reci]; ri >= 0 {
						o.simSetRecipe(p2.ri, p2.ci, reci, ri)
					}
				}
			}
		}
	}
	return improved
}

func (o *Optimizer) climbRecipes() bool {
	improved := false
	for ri := range o.rules {
		nc := o.numChefs(ri)
		for ci := 0; ci < nc; ci++ {
			for reci := 0; reci < 3; reci++ {
				curRecIdx := o.simState[ri][ci].RecipeIdxs[reci]
				currentRS := o.fastCalcRuleScore(ri)
				rk := o.getRecipeRanking(ri, ci, reci, cfg.RecipeTopN, true)
				for _, c := range rk {
					if c.recIdx == curRecIdx {
						continue
					}
					if c.score > currentRS {
						saved := o.simState[ri][ci] // value copy
						o.simSetRecipe(ri, ci, reci, c.recIdx)
						if total := o.fastCalcScore(); total > o.bestScore {
							o.bestScore = total
							o.bestSimState = cloneSimState(o.simState)
							improved = true
						} else {
							o.simState[ri][ci] = saved
							o.markDirty(ri)
						}
					}
					break
				}
			}
		}
	}
	return improved
}

func (o *Optimizer) climbRecipeSwap() bool {
	improved := false
	type rpos struct{ ri, ci, reci int }
	var positions []rpos
	for ri, rs := range o.simState {
		for ci, slot := range rs {
			for reci := 0; reci < 3; reci++ {
				if slot.RecipeIdxs[reci] >= 0 {
					positions = append(positions, rpos{ri, ci, reci})
				}
			}
		}
	}

	for i := 0; i < len(positions)-1; i++ {
		for j := i + 1; j < len(positions); j++ {
			p1, p2 := positions[i], positions[j]
			idx1 := o.simState[p1.ri][p1.ci].RecipeIdxs[p1.reci]
			idx2 := o.simState[p2.ri][p2.ci].RecipeIdxs[p2.reci]
			if idx1 < 0 || idx2 < 0 || idx1 == idx2 {
				continue
			}

			// cross-rule swap: recipe indices are relative to their own rule
			r1 := &o.rules[p1.ri].Recipes[idx1]
			r2 := &o.rules[p2.ri].Recipes[idx2]

			// skill check
			var chef1, chef2 *Chef
			if ci1 := o.simState[p1.ri][p1.ci].ChefIdx; ci1 >= 0 {
				chef1 = &o.rules[p1.ri].Chefs[ci1]
			}
			if ci2 := o.simState[p2.ri][p2.ci].ChefIdx; ci2 >= 0 {
				chef2 = &o.rules[p2.ri].Chefs[ci2]
			}
			if !chefCanCook(chef1, r2) || !chefCanCook(chef2, r1) {
				continue
			}

			// for cross-rule swap, we need the recipe to exist in the other rule
			// within the same rule, swap indices directly
			if p1.ri == p2.ri {
				saved1 := o.simState[p1.ri][p1.ci]
				saved2 := o.simState[p2.ri][p2.ci]
				o.simSetRecipe(p1.ri, p1.ci, p1.reci, idx2)
				o.simSetRecipe(p2.ri, p2.ci, p2.reci, idx1)
				if total := o.fastCalcScore(); total > o.bestScore {
					o.bestScore = total
					o.bestSimState = cloneSimState(o.simState)
					improved = true
				} else {
					o.simState[p1.ri][p1.ci] = saved1
					o.simState[p2.ri][p2.ci] = saved2
					o.markDirty(p1.ri)
				}
			}
			// cross-rule recipe swap: recipes are different objects in different rules,
			// need to find corresponding recipe index in other rule by RecipeID.
			if p1.ri != p2.ri {
				r2InR1 := o.findRecipeIdx(p1.ri, r2.RecipeID)
				r1InR2 := o.findRecipeIdx(p2.ri, r1.RecipeID)
				if r2InR1 < 0 || r1InR2 < 0 {
					continue
				}
				saved1 := o.simState[p1.ri][p1.ci]
				saved2 := o.simState[p2.ri][p2.ci]
				o.simSetRecipe(p1.ri, p1.ci, p1.reci, r2InR1)
				o.simSetRecipe(p2.ri, p2.ci, p2.reci, r1InR2)
				if total := o.fastCalcScore(); total > o.bestScore {
					o.bestScore = total
					o.bestSimState = cloneSimState(o.simState)
					improved = true
				} else {
					o.simState[p1.ri][p1.ci] = saved1
					o.simState[p2.ri][p2.ci] = saved2
					o.markDirty(p1.ri)
					o.markDirty(p2.ri)
				}
			}
		}
	}
	return improved
}

func (o *Optimizer) findChefIdx(ri int, chefID int) int {
	for i, c := range o.rules[ri].Chefs {
		if c.ChefID == chefID {
			return i
		}
	}
	return -1
}

func (o *Optimizer) findRecipeIdx(ri, recipeID int) int {
	for i, r := range o.rules[ri].Recipes {
		if r.RecipeID == recipeID {
			return i
		}
	}
	return -1
}

// ── Climbing phase (multi-round) ────────────────────────────────────

func (o *Optimizer) runClimbing(startRound int) {
	for round := startRound; round < cfg.MaxRounds; round++ {
		o.setSimState(cloneSimState(o.bestSimState))
		c1 := o.climbChefs()
		o.setSimState(cloneSimState(o.bestSimState))
		c2 := o.climbChefSwap()
		o.setSimState(cloneSimState(o.bestSimState))
		c3 := o.climbRecipes()
		o.setSimState(cloneSimState(o.bestSimState))
		c4 := o.climbRecipeSwap()
		if !c1 && !c2 && !c3 && !c4 {
			break
		}
	}
}

// ── Cross-guest reassignment ────────────────────────────────────────

func (o *Optimizer) crossGuestReassign(activeRules []int) bool {
	if len(activeRules) < 2 {
		return false
	}
	improved := false

	for _, targetRule := range activeRules {
		nc := o.numChefs(targetRule)
		rule := &o.rules[targetRule]

		// Plan A: clear target guest, greedy refill
		o.setSimState(cloneSimState(o.bestSimState))
		for ci := 0; ci < nc; ci++ {
			o.clearSlot(targetRule, ci)
		}
		o.greedyFillGuest(targetRule)
		if score := o.fastCalcScore(); score >= int(float64(o.bestScore)*0.90) {
			o.quickRefine(activeRules, false)
		}
		if s := o.fastCalcScore(); s > o.bestScore {
			o.bestScore = s
			o.bestSimState = cloneSimState(o.simState)
			improved = true
		}

		// Plan B: per-position, try different seed recipes
		for seedPos := 0; seedPos < nc; seedPos++ {
			o.setSimState(cloneSimState(o.bestSimState))
			o.clearSlot(targetRule, seedPos)

			topRecipes := o.getRecipeRanking(targetRule, seedPos, 0, 5, true)
			for _, rc := range topRecipes {
				o.setSimState(cloneSimState(o.bestSimState))
				o.clearSlot(targetRule, seedPos)
				o.simSetRecipe(targetRule, seedPos, 0, rc.recIdx)

				ck := o.getChefRanking(targetRule, seedPos, true)
				usedIDs := o.getUsedChefIdxs(targetRule, seedPos)
				for _, c := range ck {
					if !c.skillOk || usedIDs[rule.Chefs[c.chefIdx].ChefID] {
						continue
					}
					o.simSetChef(targetRule, seedPos, c.chefIdx)
					break
				}
				o.greedyFillRecipes(targetRule, seedPos)

				if pre := o.fastCalcScore(); pre < int(float64(o.bestScore)*0.90) {
					continue
				}
				o.quickRefine(activeRules, false)
				if s := o.fastCalcScore(); s > o.bestScore {
					o.bestScore = s
					o.bestSimState = cloneSimState(o.simState)
					improved = true
				}
			}
		}
	}

	if improved {
		o.setSimState(cloneSimState(o.bestSimState))
		o.quickRefine(activeRules, false)
		if s := o.fastCalcScore(); s > o.bestScore {
			o.bestScore = s
			o.bestSimState = cloneSimState(o.simState)
		}
	}
	o.setSimState(cloneSimState(o.bestSimState))
	return improved
}

// ── Full guest rebuild ──────────────────────────────────────────────

func (o *Optimizer) fullGuestRebuild(activeRules []int) bool {
	improved := false
	for _, targetRule := range activeRules {
		rule := &o.rules[targetRule]
		nc := o.numChefs(targetRule)

		// collect chef IDs used by other guests
		otherUsedIDs := make(map[int]bool)
		for ri, rs := range o.bestSimState {
			if ri == targetRule {
				continue
			}
			for _, slot := range rs {
				if slot.ChefIdx >= 0 {
					otherUsedIDs[o.rules[ri].Chefs[slot.ChefIdx].ChefID] = true
				}
			}
		}

		type chefEntry struct {
			idx    int
			rarity int
		}
		var available []chefEntry
		for ci := range rule.Chefs {
			if !rule.Chefs[ci].Got || otherUsedIDs[rule.Chefs[ci].ChefID] {
				continue
			}
			available = append(available, chefEntry{ci, rule.Chefs[ci].Rarity})
		}
		sort.Slice(available, func(i, j int) bool { return available[i].rarity > available[j].rarity })

		triedCombos := make(map[string]bool)

		for startPos := 0; startPos < nc; startPos++ {
			lim := 5
			if lim > len(available) {
				lim = len(available)
			}
			for fi := 0; fi < lim; fi++ {
				o.setSimState(cloneSimState(o.bestSimState))
				for ci := 0; ci < nc; ci++ {
					o.clearSlot(targetRule, ci)
				}
				o.simSetChef(targetRule, startPos, available[fi].idx)
				rk := o.getRecipeRanking(targetRule, startPos, 0, 1, true)
				if len(rk) > 0 {
					o.simSetRecipe(targetRule, startPos, 0, rk[0].recIdx)
				}
				o.greedyFillRecipes(targetRule, startPos)
				for ci := 0; ci < nc; ci++ {
					if ci != startPos {
						o.greedyFillPosition(targetRule, ci)
					}
				}
				o.quickRefine(activeRules, true)

				// dedup by chef combo
				ids := make([]int, nc)
				for ci := 0; ci < nc; ci++ {
					ids[ci] = o.simState[targetRule][ci].ChefIdx
				}
				sort.Ints(ids)
				key := fmt.Sprint(ids)
				if triedCombos[key] {
					continue
				}
				triedCombos[key] = true

				if s := o.fastCalcScore(); s > o.bestScore {
					o.bestScore = s
					o.bestSimState = cloneSimState(o.simState)
					improved = true
				}
			}
		}
	}
	o.setSimState(cloneSimState(o.bestSimState))
	return improved
}

// ── Seed generation ─────────────────────────────────────────────────

type seedCandidate struct {
	state SimState
	score int
}

func (o *Optimizer) generateSeeds() []seedCandidate {
	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}

	var candidates []seedCandidate

	for _, mainRule := range activeRules {
		nc := o.numChefs(mainRule)
		rule := &o.rules[mainRule]
		for seedPos := 0; seedPos < nc; seedPos++ {
			o.initSimState()
			topRecipes := o.getRecipeRanking(mainRule, seedPos, 0, cfg.RecipeSeedK, true)

			for _, seedRC := range topRecipes {
				o.initSimState()
				o.simSetRecipe(mainRule, seedPos, 0, seedRC.recIdx)
				chefRanking := o.getChefRanking(mainRule, seedPos, true)

				chefsTried := 0
				for _, cr := range chefRanking {
					if chefsTried >= cfg.ChefPerSeed {
						break
					}
					if !cr.skillOk {
						continue
					}
					usedIDs := o.getUsedChefIdxs(mainRule, seedPos)
					if usedIDs[rule.Chefs[cr.chefIdx].ChefID] {
						continue
					}
					chefsTried++

					o.initSimState()
					o.simSetRecipe(mainRule, seedPos, 0, seedRC.recIdx)
					o.simSetChef(mainRule, seedPos, cr.chefIdx)
					o.greedyFillRecipes(mainRule, seedPos)

					for ci := 0; ci < nc; ci++ {
						if ci != seedPos {
							o.greedyFillPosition(mainRule, ci)
						}
					}
					for _, otherRule := range activeRules {
						if otherRule != mainRule {
							o.greedyFillGuest(otherRule)
						}
					}

					score := o.fastCalcScore()
					candidates = append(candidates, seedCandidate{
						state: cloneSimState(o.simState),
						score: score,
					})
				}
			}
		}
	}
	return candidates
}

// ── Diversity selection ─────────────────────────────────────────────

func selectDiverseSeeds(cands []seedCandidate, maxSeeds int) []SimState {
	if len(cands) == 0 {
		return nil
	}

	chefSet := func(st SimState) map[int]bool {
		s := make(map[int]bool)
		for _, rs := range st {
			for _, slot := range rs {
				if slot.ChefIdx >= 0 {
					s[slot.ChefIdx] = true
				}
			}
		}
		return s
	}

	overlap := func(a, b map[int]bool) float64 {
		shared, total := 0, 0
		for k := range a {
			total++
			if b[k] {
				shared++
			}
		}
		if total == 0 {
			return 0
		}
		return float64(shared) / float64(total)
	}

	var seeds []SimState
	var sets []map[int]bool

	seeds = append(seeds, cloneSimState(cands[0].state))
	sets = append(sets, chefSet(cands[0].state))

	// pass 1: diversity threshold 0.67
	for i := 1; i < len(cands) && len(seeds) < maxSeeds; i++ {
		cs := chefSet(cands[i].state)
		similar := false
		for _, ss := range sets {
			if overlap(cs, ss) > 0.67 {
				similar = true
				break
			}
		}
		if !similar {
			seeds = append(seeds, cloneSimState(cands[i].state))
			sets = append(sets, cs)
		}
	}

	// pass 2: fill remaining slots
	for i := 1; i < len(cands) && len(seeds) < maxSeeds; i++ {
		cs := chefSet(cands[i].state)
		dup := false
		for _, ss := range sets {
			if overlap(cs, ss) >= 1.0 {
				dup = true
				break
			}
		}
		if !dup {
			seeds = append(seeds, cloneSimState(cands[i].state))
			sets = append(sets, cs)
		}
	}
	return seeds
}

// ── Fingerprint dedup ───────────────────────────────────────────────

func dedupSeeds(seeds []SimState) []SimState {
	seen := make(map[string]bool)
	var out []SimState
	for _, s := range seeds {
		fp := stateFingerprint(s)
		if !seen[fp] {
			seen[fp] = true
			out = append(out, s)
		}
	}
	return out
}

func stateFingerprint(s SimState) string {
	n := 0
	for _, rs := range s {
		n += len(rs) * 4 // chefIdx + 3 recipeIdxs
	}
	buf := make([]byte, 0, n*4)
	for _, rs := range s {
		for _, slot := range rs {
			v := slot.ChefIdx
			buf = append(buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			for reci := 0; reci < 3; reci++ {
				v = slot.RecipeIdxs[reci]
				buf = append(buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
			}
		}
	}
	return string(buf)
}

// ── Deep search per seed (runs in its own goroutine) ────────────────

func (o *Optimizer) deepSearchSeed(seed SimState) (int, SimState) {
	o.setSimState(cloneSimState(seed))
	o.bestScore = o.fastCalcScore()
	o.bestSimState = cloneSimState(o.simState)

	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}

	// full climbing
	o.runClimbing(0)
	if Verbose {
		fmt.Fprintf(logw(), "[verbose/deep] after climbing: %d\n", o.bestScore)
	}

	// cross-guest (always attempt, no early exit)
	for crossRound := 0; crossRound < 2; crossRound++ {
		if !o.crossGuestReassign(activeRules) {
			break
		}
		o.setSimState(cloneSimState(o.bestSimState))
		o.climbRecipeSwap()
		if s := o.fastCalcScore(); s > o.bestScore {
			o.bestScore = s
			o.bestSimState = cloneSimState(o.simState)
		}
	}
	if Verbose {
		fmt.Fprintf(logw(), "[verbose/deep] after cross-guest: %d\n", o.bestScore)
	}

	// full guest rebuild
	o.fullGuestRebuild(activeRules)
	if Verbose {
		fmt.Fprintf(logw(), "[verbose/deep] after guest rebuild: %d\n", o.bestScore)
	}

	// final recipe swap + full climbing
	o.setSimState(cloneSimState(o.bestSimState))
	o.climbRecipeSwap()
	if s := o.fastCalcScore(); s > o.bestScore {
		o.bestScore = s
		o.bestSimState = cloneSimState(o.simState)
	}
	o.runClimbing(0)

	return o.bestScore, o.bestSimState
}

// ── Clone optimizer for goroutine use ───────────────────────────────

func (o *Optimizer) clone() *Optimizer {
	c := &Optimizer{
		rules:       o.rules, // shared read-only
		gameData:    o.gameData,
		menusByRule: o.menusByRule,
	}
	c.initSimState()
	return c
}

// ── Main entry point ────────────────────────────────────────────────

// Optimize runs the full multi-phase search and returns the best score, state, and elapsed time.
func (o *Optimizer) Optimize(targetScore int) (int, SimState, time.Duration) {
	start := time.Now()

	fmt.Fprintf(logw(), "[init] rules=%d, target=%d\n", len(o.rules), targetScore)

	// Phase 1: seed generation
	candidates := o.generateSeeds()
	fmt.Fprintf(logw(), "[seed] candidates=%d\n", len(candidates))
	if Verbose {
		fmt.Fprintf(logw(), "[verbose] generated %d seed candidates\n", len(candidates))
	}
	if len(candidates) == 0 {
		return 0, nil, time.Since(start)
	}

	// Phase 2: batch refine ALL candidates (no top-20 limit)
	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}
	for i := range candidates {
		o.setSimState(candidates[i].state)
		o.quickRefine(activeRules, false)
		candidates[i].score = o.fastCalcScore()
		candidates[i].state = cloneSimState(o.simState)
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	fmt.Fprintf(logw(), "[refine] best=%d, worst=%d\n",
		candidates[0].score, candidates[len(candidates)-1].score)

	if Verbose {
		limit := len(candidates)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(logw(), "[verbose] seed#%d score after refine: %d\n", i, candidates[i].score)
		}
	}

	// Phase 3: diversity selection + dedup
	seeds := selectDiverseSeeds(candidates, cfg.MaxDiverseSeeds)
	seeds = dedupSeeds(seeds)
	fmt.Fprintf(logw(), "[diversity] seeds=%d\n", len(seeds))

	// Phase 4: deep search ALL seeds in parallel goroutines
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers > len(seeds) {
		numWorkers = len(seeds)
	}

	type result struct {
		score int
		state SimState
		idx   int
	}
	resultCh := make(chan result, len(seeds))
	seedCh := make(chan int, len(seeds))
	for i := range seeds {
		seedCh <- i
	}
	close(seedCh)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker := o.clone()
			for idx := range seedCh {
				score, state := worker.deepSearchSeed(seeds[idx])
				resultCh <- result{score, state, idx}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	globalBest := 0
	var globalBestState SimState
	winnerIdx := -1
	for r := range resultCh {
		fmt.Fprintf(logw(), "[deep] seed#%d done, score=%d\n", r.idx, r.score)
		if r.score > globalBest {
			globalBest = r.score
			globalBestState = r.state
			winnerIdx = r.idx
		}
	}

	elapsed := time.Since(start)
	if Verbose {
		fmt.Fprintf(logw(), "[verbose] winning seed: #%d with score %d\n", winnerIdx, globalBest)
	}
	fmt.Fprintf(logw(), "[done] best=%d, elapsed=%v\n", globalBest, elapsed)
	return globalBest, globalBestState, elapsed
}

func logw() *os.File { return os.Stderr }

func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
