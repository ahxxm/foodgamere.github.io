package main

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

type Optimizer struct {
	cfg      Config
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

	// reusable scratch slices to avoid per-call allocation (indexed by ID, not hash)
	scratchUsedChefIDs  []bool // indexed by ChefID
	scratchUsedRecIDs   []bool // indexed by RecipeID
	scratchUsedRecLocal []bool // indexed by recipe index within a rule
	ruleMatIndex        [][]int // per-rule material index, built once at init

	// reusable scratch buffers for calcRuleScoreReuse
	scratch ScratchBuffers

	// reusable scratch buffers for getRecipeRanking / getChefRanking
	scratchPhase1        []rough
	scratchRecipeResults []recipeRank
	scratchChefResults   []chefRank
}

func NewOptimizer(contest *Contest, gameData *GameData, cfg Config) *Optimizer {
	maxChefID, maxRecipeID, maxRecPerRule := 0, 0, 0
	for ri := range contest.Rules {
		rule := &contest.Rules[ri]
		for ci := range rule.Chefs {
			if rule.Chefs[ci].ChefID > maxChefID {
				maxChefID = rule.Chefs[ci].ChefID
			}
		}
		for ri2 := range rule.Recipes {
			if rule.Recipes[ri2].RecipeID > maxRecipeID {
				maxRecipeID = rule.Recipes[ri2].RecipeID
			}
		}
		if len(rule.Recipes) > maxRecPerRule {
			maxRecPerRule = len(rule.Recipes)
		}
	}

	ruleMatIdx := make([][]int, len(contest.Rules))
	for ri := range contest.Rules {
		ruleMatIdx[ri] = buildMatIndex(contest.Rules[ri].Materials, nil)
	}

	o := &Optimizer{
		cfg:                 cfg,
		rules:               contest.Rules,
		gameData:            gameData,
		scratchUsedChefIDs:  make([]bool, maxChefID+1),
		scratchUsedRecIDs:   make([]bool, maxRecipeID+1),
		scratchUsedRecLocal: make([]bool, maxRecPerRule),
		ruleMatIndex:        ruleMatIdx,
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
	qty := getRecipeQuantity(recipe, rule.Materials, rule, chef, o.ruleMatIndex[ri])
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

func (o *Optimizer) fastCalcScore() int {
	total := 0
	for ri := range o.rules {
		if o.dirtyRules[ri] {
			o.ruleScoreCache[ri] = calcRuleScoreReuse(o.rules, o.simState, ri, o.gameData, &o.scratch)
			o.dirtyRules[ri] = false
		}
		total += o.ruleScoreCache[ri]
	}
	return total
}

func (o *Optimizer) fastCalcRuleScore(ri int) int {
	if o.dirtyRules[ri] {
		o.ruleScoreCache[ri] = calcRuleScoreReuse(o.rules, o.simState, ri, o.gameData, &o.scratch)
		o.dirtyRules[ri] = false
	}
	return o.ruleScoreCache[ri]
}

func (o *Optimizer) getUsedChefIdxs(excludeRI, excludeCI int) []bool {
	used := o.scratchUsedChefIDs
	for i := range used {
		used[i] = false
	}
	for ri, rs := range o.simState {
		for ci, slot := range rs {
			if ri == excludeRI && ci == excludeCI {
				continue
			}
			if slot.ChefIdx >= 0 {
				used[o.rules[ri].Chefs[slot.ChefIdx].ChefID] = true
			}
		}
	}
	return used
}

func (o *Optimizer) getUsedRecipeIdxs(ri, excludeCI, excludeRec int) []bool {
	usedIDs := o.scratchUsedRecIDs
	for i := range usedIDs {
		usedIDs[i] = false
	}
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
	used := o.scratchUsedRecLocal
	for i := range used {
		used[i] = false
	}
	for i, r := range o.rules[ri].Recipes {
		if r.RecipeID < len(usedIDs) && usedIDs[r.RecipeID] {
			used[i] = true
		}
	}
	return used
}

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

type rough struct {
	recIdx int
	est    int
}

type recipeRank struct {
	recIdx int
	score  int
}

func (o *Optimizer) getRecipeRanking(ri, ci, recSlot, topK int) []recipeRank {
	rule := &o.rules[ri]
	menus := o.menusByRule[ri]
	usedIdxs := o.getUsedRecipeIdxs(ri, ci, recSlot)

	slot := &o.simState[ri][ci]
	var chef *Chef
	if slot.ChefIdx >= 0 {
		chef = &rule.Chefs[slot.ChefIdx]
	}

	savedIdx := slot.RecipeIdxs[recSlot]
	savedQty := slot.Quantities[recSlot]
	savedMax := slot.MaxQty[recSlot]

	// Phase 1: rough estimate (price * qty)
	o.scratchPhase1 = o.scratchPhase1[:0]
	for _, recIdx := range menus {
		if usedIdxs[recIdx] {
			continue
		}
		recipe := &rule.Recipes[recIdx]
		if !chefCanCook(chef, recipe) {
			continue
		}
		qty := getRecipeQuantity(recipe, rule.Materials, rule, chef, o.ruleMatIndex[ri])
		if rule.DisableMultiCookbook && qty > 1 {
			qty = 1
		}
		est := int(recipe.Price * float64(qty))
		o.scratchPhase1 = append(o.scratchPhase1, rough{recIdx, est})
	}
	phase1 := o.scratchPhase1
	slices.SortFunc(phase1, func(a, b rough) int { return b.est - a.est })

	// Phase 2: intent-aware re-ranking on top candidates
	// Only runs when the position has recipe-dependent intents (CookSkill, Rarity, etc.)
	needPhase2 := hasRecipeDependentIntents(rule, ci, o.gameData)
	var phase2Size int
	if needPhase2 {
		if topK > 0 && topK <= 3 {
			phase2Size = 8
		} else {
			phase2Size = 15
		}
	}

	if needPhase2 && phase2Size > 0 && len(phase1) > 0 {
		p2Limit := phase2Size
		if p2Limit > len(phase1) {
			p2Limit = len(phase1)
		}

		slots := resolveRuleState(rule, o.simState[ri])
		customArr := buildCustomArr(slots)
		applyChefDataForRule(rule, slots, getPartialChefAdds(customArr, rule))
		slotIdx := 3*ci + recSlot

		total := 3 * len(slots)
		intentAdds := make([][]*Intent, total)
		partialRecipeAdds := make([][]PartialAdd, total)

		for i := 0; i < p2Limit; i++ {
			recIdx := phase1[i].recIdx
			recipe := &rule.Recipes[recIdx]
			qty := getRecipeQuantity(recipe, rule.Materials, rule, chef, o.ruleMatIndex[ri])
			if rule.DisableMultiCookbook && qty > 1 {
				qty = 1
			}

			slots[ci].recipes[recSlot] = recipeSlot{Data: recipe, Quantity: qty, Max: qty}

			buildCustomArrInto(slots, customArr)
			for k := range intentAdds {
				intentAdds[k] = intentAdds[k][:0]
			}
			getIntentAddsInto(ri, customArr, o.gameData, o.rules, intentAdds)
			for k := range partialRecipeAdds {
				partialRecipeAdds[k] = partialRecipeAdds[k][:0]
			}
			getPartialRecipeAddsInto(customArr, rule, partialRecipeAdds)

			var ia []*Intent
			if slotIdx < len(intentAdds) {
				ia = intentAdds[slotIdx]
			}
			var pa []PartialAdd
			if slotIdx < len(partialRecipeAdds) {
				pa = partialRecipeAdds[slotIdx]
			}

			totalScore, _ := getRecipeScore(
				slots[ci].chefObj, slots[ci].equipObj,
				recipe, qty, rule,
				&slots[ci].recipes, pa, ia,
			)
			actAdd := recipe.ActivityAddition
			est2 := int(math.Ceil(toFixed2(float64(totalScore) * (1 + actAdd/100))))
			phase1[i].est = est2
		}

		if savedIdx >= 0 {
			slots[ci].recipes[recSlot] = recipeSlot{
				Data: &rule.Recipes[savedIdx], Quantity: savedQty, Max: savedMax,
			}
		} else {
			slots[ci].recipes[recSlot] = recipeSlot{}
		}

		slices.SortFunc(phase1, func(a, b rough) int { return b.est - a.est })
	}

	// Phase 3: precise scoring on top candidates (no artificial cap)
	limit := len(phase1)
	if cap := max(topK*2, o.cfg.PreFilterTop); limit > cap {
		limit = cap
	}

	o.scratchRecipeResults = o.scratchRecipeResults[:0]
	for i := 0; i < limit; i++ {
		o.simSetRecipe(ri, ci, recSlot, phase1[i].recIdx)
		score := o.fastCalcRuleScore(ri)
		o.scratchRecipeResults = append(o.scratchRecipeResults, recipeRank{phase1[i].recIdx, score})
	}

	slot.RecipeIdxs[recSlot] = savedIdx
	slot.Quantities[recSlot] = savedQty
	slot.MaxQty[recSlot] = savedMax
	o.markDirty(ri)

	partialTopK(o.scratchRecipeResults, topK, func(a, b recipeRank) int { return b.score - a.score })
	n := len(o.scratchRecipeResults)
	if topK > 0 && n > topK {
		n = topK
	}
	// Return a copy — callers may hold references across nested getRecipeRanking calls
	out := make([]recipeRank, n)
	copy(out, o.scratchRecipeResults[:n])
	return out
}

func isRecipeDepCondition(ct IntentCondType) bool {
	switch ct {
	case IntCondCookSkill, IntCondCondimentSkill, IntCondRarity, IntCondRank, IntCondGroup:
		return true
	}
	return false
}

func hasRecipeDependentIntents(rule *Rule, ci int, gameData *GameData) bool {
	if rule.Satiety == 0 || gameData == nil {
		return false
	}

	for _, gbID := range rule.GlobalBuffList {
		if gbID < len(gameData.BuffByID) {
			if buff := gameData.BuffByID[gbID]; buff != nil && isRecipeDepCondition(buff.ConditionType) {
				return true
			}
		}
	}

	if ci >= len(rule.IntentList) {
		return false
	}
	for _, intentID := range rule.IntentList[ci] {
		if intentID >= len(gameData.IntentByID) {
			continue
		}
		intent := gameData.IntentByID[intentID]
		if intent == nil {
			continue
		}
		if isRecipeDepCondition(intent.ConditionType) {
			return true
		}
		// CreateIntent: check child intent
		if intent.EffectType == IntEffCreateIntent {
			childID := int(intent.EffectValue)
			if childID < len(gameData.IntentByID) {
				if child := gameData.IntentByID[childID]; child != nil && isRecipeDepCondition(child.ConditionType) {
					return true
				}
			}
		}
		// CreateBuff: check child buff
		if intent.EffectType == IntEffCreateBuff {
			buffID := int(intent.EffectValue)
			if buffID < len(gameData.BuffByID) {
				if buff := gameData.BuffByID[buffID]; buff != nil && isRecipeDepCondition(buff.ConditionType) {
					return true
				}
			}
		}
	}
	return false
}

type chefRank struct {
	chefIdx int
	score   int
	skillOk bool
}

func (o *Optimizer) getChefRanking(ri, ci int) []chefRank {
	rule := &o.rules[ri]
	usedIDs := o.getUsedChefIdxs(ri, ci)
	slot := &o.simState[ri][ci]

	savedChefIdx := slot.ChefIdx

	hasRecipe := slot.RecipeIdxs[0] >= 0 || slot.RecipeIdxs[1] >= 0 || slot.RecipeIdxs[2] >= 0

	o.scratchChefResults = o.scratchChefResults[:0]
	if !hasRecipe {
		for idx := range rule.Chefs {
			if !rule.Chefs[idx].Got {
				continue
			}
			o.scratchChefResults = append(o.scratchChefResults, chefRank{idx, rule.Chefs[idx].Rarity, true})
		}
	} else {
		for idx := range rule.Chefs {
			ch := &rule.Chefs[idx]
			if !ch.Got {
				continue
			}
			if usedIDs[ch.ChefID] {
				continue
			}
			ok := true
			for reci := 0; reci < 3; reci++ {
				ri2 := slot.RecipeIdxs[reci]
				if ri2 >= 0 && !chefCanCook(ch, &rule.Recipes[ri2]) {
					ok = false
					break
				}
			}
			if !ok {
				o.scratchChefResults = append(o.scratchChefResults, chefRank{idx, -1, false})
				continue
			}
			o.simSetChef(ri, ci, idx)
			score := o.fastCalcRuleScore(ri)
			o.scratchChefResults = append(o.scratchChefResults, chefRank{idx, score, true})
		}
		o.simSetChef(ri, ci, savedChefIdx)
	}

	slices.SortFunc(o.scratchChefResults, func(a, b chefRank) int { return b.score - a.score })
	return o.scratchChefResults
}

func (o *Optimizer) greedyFillRecipes(ri, ci int) {
	for reci := 0; reci < 3; reci++ {
		if o.simState[ri][ci].RecipeIdxs[reci] >= 0 {
			continue
		}
		rk := o.getRecipeRanking(ri, ci, reci, 1)
		if len(rk) > 0 {
			o.simSetRecipe(ri, ci, reci, rk[0].recIdx)
		}
	}
}

func (o *Optimizer) greedyFillPosition(ri, ci int) {
	rk := o.getRecipeRanking(ri, ci, 0, 1)
	if len(rk) > 0 {
		o.simSetRecipe(ri, ci, 0, rk[0].recIdx)
	}
	ck := o.getChefRanking(ri, ci)
	usedIDs := o.getUsedChefIdxs(ri, ci)
	rule := &o.rules[ri]
	for _, c := range ck {
		if !c.skillOk || usedIDs[rule.Chefs[c.chefIdx].ChefID] {
			continue
		}
		o.simSetChef(ri, ci, c.chefIdx)
		break
	}
	o.greedyFillRecipes(ri, ci)
}

func (o *Optimizer) greedyFillGuest(ri int) {
	nc := o.numChefs(ri)
	for ci := 0; ci < nc; ci++ {
		o.greedyFillPosition(ri, ci)
	}
}

func (o *Optimizer) quickRefine(activeRules []int, light bool) {
	maxIter := o.cfg.RefineIter
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
					rk := o.getRecipeRanking(ri, ci, reci, 1)
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
				ck := o.getChefRanking(ri, ci)
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
					rk := o.getRecipeRanking(ri, ci, reci, 1)
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

func (o *Optimizer) climbChefs() bool {
	improved := false
	for ri := range o.rules {
		rule := &o.rules[ri]
		nc := o.numChefs(ri)
		for ci := 0; ci < nc; ci++ {
			curChefIdx := o.simState[ri][ci].ChefIdx
			usedIDs := o.getUsedChefIdxs(ri, ci)
			currentRS := o.fastCalcRuleScore(ri)

			ck := o.getChefRanking(ri, ci)
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

			// swap — verify skill compatibility before scoring
			chef1New := &o.rules[p1.ri].Chefs[swapIdx1]
			chef2New := &o.rules[p2.ri].Chefs[swapIdx2]
			slot1 := &o.simState[p1.ri][p1.ci]
			slot2 := &o.simState[p2.ri][p2.ci]
			canSwap := true
			for reci := 0; reci < 3; reci++ {
				if ri := slot1.RecipeIdxs[reci]; ri >= 0 && !chefCanCook(chef1New, &o.rules[p1.ri].Recipes[ri]) {
					canSwap = false
					break
				}
				if ri := slot2.RecipeIdxs[reci]; ri >= 0 && !chefCanCook(chef2New, &o.rules[p2.ri].Recipes[ri]) {
					canSwap = false
					break
				}
			}
			if !canSwap {
				continue
			}

			o.simSetChef(p1.ri, p1.ci, swapIdx1)
			o.simSetChef(p2.ri, p2.ci, swapIdx2)
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
				rk := o.getRecipeRanking(ri, ci, reci, o.cfg.RecipeTopN)
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

func (o *Optimizer) climbJointChefRecipe() bool {
	improved := false
	for ri := range o.rules {
		rule := &o.rules[ri]
		nc := o.numChefs(ri)
		for ci := 0; ci < nc; ci++ {
			savedRule := make(RuleState, nc)
			copy(savedRule, o.simState[ri])
			usedIDs := o.getUsedChefIdxs(ri, ci)
			ck := o.getChefRanking(ri, ci)
			accepted := false
			tried := 0
			for _, cr := range ck {
				if tried >= 3 {
					break
				}
				if !cr.skillOk || cr.chefIdx == savedRule[ci].ChefIdx || usedIDs[rule.Chefs[cr.chefIdx].ChefID] {
					continue
				}
				tried++
				copy(o.simState[ri], savedRule)
				o.markDirty(ri)
				o.simSetChef(ri, ci, cr.chefIdx)
				for reci := 0; reci < 3; reci++ {
					o.simSetRecipe(ri, ci, reci, -1)
				}
				for reci := 0; reci < 3; reci++ {
					rk := o.getRecipeRanking(ri, ci, reci, 1)
					if len(rk) > 0 {
						o.simSetRecipe(ri, ci, reci, rk[0].recIdx)
					}
				}
				for pass := 0; pass < 2; pass++ {
					changed := false
					for ci2 := 0; ci2 < nc; ci2++ {
						for reci := 0; reci < 3; reci++ {
							curIdx := o.simState[ri][ci2].RecipeIdxs[reci]
							rk := o.getRecipeRanking(ri, ci2, reci, 1)
							if len(rk) > 0 && rk[0].recIdx != curIdx {
								o.simSetRecipe(ri, ci2, reci, rk[0].recIdx)
								changed = true
							}
						}
					}
					if !changed {
						break
					}
				}
				if total := o.fastCalcScore(); total > o.bestScore {
					o.bestScore = total
					o.bestSimState = cloneSimState(o.simState)
					improved = true
					accepted = true
					break
				}
			}
			if !accepted {
				copy(o.simState[ri], savedRule)
				o.markDirty(ri)
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

func (o *Optimizer) runClimbing() {
	for round := 0; round < o.cfg.MaxRounds; round++ {
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

			topRecipes := o.getRecipeRanking(targetRule, seedPos, 0, 5)
			for _, rc := range topRecipes {
				o.setSimState(cloneSimState(o.bestSimState))
				o.clearSlot(targetRule, seedPos)
				o.simSetRecipe(targetRule, seedPos, 0, rc.recIdx)

				ck := o.getChefRanking(targetRule, seedPos)
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
			topRecipes := o.getRecipeRanking(mainRule, seedPos, 0, o.cfg.RecipeSeedK)

			for _, seedRC := range topRecipes {
				o.initSimState()
				o.simSetRecipe(mainRule, seedPos, 0, seedRC.recIdx)
				chefRankScratch := o.getChefRanking(mainRule, seedPos)
				// Copy: greedyFillPosition below calls getChefRanking, reusing the scratch
				chefRanking := make([]chefRank, len(chefRankScratch))
				copy(chefRanking, chefRankScratch)

				chefsTried := 0
				for _, cr := range chefRanking {
					if chefsTried >= o.cfg.ChefPerSeed {
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

func (o *Optimizer) generateAuraSeeds() []seedCandidate {
	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}

	var candidates []seedCandidate

	for _, mainRule := range activeRules {
		rule := &o.rules[mainRule]
		nc := o.numChefs(mainRule)

		type auraChef struct {
			idx   int
			score float64
		}
		var auras []auraChef
		for ci := range rule.Chefs {
			ch := &rule.Chefs[ci]
			if !ch.Got {
				continue
			}
			if ch.ChefID >= len(rule.CalPartialChefSet) || !rule.CalPartialChefSet[ch.ChefID] {
				continue
			}
			aScore := 0.0
			for _, eff := range ch.UltimateSkillEffect {
				if (eff.Condition == CondScopePartial || eff.Condition == CondScopeNext) && isBasicPriceUseType(eff.Type) {
					aScore += math.Abs(eff.Value)
				}
			}
			if aScore > 0 {
				auras = append(auras, auraChef{ci, aScore})
			}
		}
		if len(auras) == 0 {
			continue
		}
		slices.SortFunc(auras, func(a, b auraChef) int { return cmp.Compare(b.score, a.score) })
		if len(auras) > 3 {
			auras = auras[:3]
		}

		for _, ac := range auras {
			for seedPos := 0; seedPos < nc; seedPos++ {
				o.initSimState()
				o.simSetChef(mainRule, seedPos, ac.idx)
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

	return candidates
}

func (o *Optimizer) generateMultiSkillSeeds() []seedCandidate {
	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}

	var candidates []seedCandidate

	for _, mainRule := range activeRules {
		rule := &o.rules[mainRule]
		nc := o.numChefs(mainRule)

		skillSet := map[CookSkill]bool{}

		for _, intentIDs := range rule.IntentList {
			for _, intentID := range intentIDs {
				if intentID >= len(o.gameData.IntentByID) {
					continue
				}
				intent := o.gameData.IntentByID[intentID]
				if intent == nil {
					continue
				}
				if intent.ConditionType == IntCondGroup && intent.CondValSkill != CookNone {
					skillSet[intent.CondValSkill] = true
				}
				if intent.EffectType == IntEffCreateBuff {
					buffID := int(intent.EffectValue)
					if buffID < len(o.gameData.BuffByID) {
						if buff := o.gameData.BuffByID[buffID]; buff != nil && buff.ConditionType == IntCondGroup && buff.CondValSkill != CookNone {
							skillSet[buff.CondValSkill] = true
						}
					}
				}
			}
		}
		for _, gbID := range rule.GlobalBuffList {
			if gbID < len(o.gameData.BuffByID) {
				if buff := o.gameData.BuffByID[gbID]; buff != nil && buff.ConditionType == IntCondGroup && buff.CondValSkill != CookNone {
					skillSet[buff.CondValSkill] = true
				}
			}
		}

		if len(skillSet) < 2 {
			continue
		}

		var skills []CookSkill
		for s := range skillSet {
			skills = append(skills, s)
		}
		slices.SortFunc(skills, func(a, b CookSkill) int { return int(a) - int(b) })

		type combo struct{ skills []CookSkill }
		var combos []combo
		for i := 0; i < len(skills)-1; i++ {
			for j := i + 1; j < len(skills); j++ {
				combos = append(combos, combo{[]CookSkill{skills[i], skills[j]}})
			}
		}
		if len(skills) > 2 {
			combos = append(combos, combo{skills})
		}

		for _, cb := range combos {
			var matching []int
			for _, recIdx := range o.menusByRule[mainRule] {
				recipe := &rule.Recipes[recIdx]
				hasAll := true
				for _, sk := range cb.skills {
					if !recipeHasSkill(recipe, sk) {
						hasAll = false
						break
					}
				}
				if hasAll {
					matching = append(matching, recIdx)
				}
			}
			if len(matching) < 3 {
				continue
			}
			slices.SortFunc(matching, func(a, b int) int {
				return cmp.Compare(rule.Recipes[b].Price, rule.Recipes[a].Price)
			})
			if len(matching) > 9 {
				matching = matching[:9]
			}

			for seedPos := 0; seedPos < nc; seedPos++ {
				if len(matching) < 3 {
					continue
				}

				o.initSimState()
				o.simSetRecipe(mainRule, seedPos, 0, matching[0])
				o.simSetRecipe(mainRule, seedPos, 1, matching[1])
				o.simSetRecipe(mainRule, seedPos, 2, matching[2])

				ck := o.getChefRanking(mainRule, seedPos)
				usedIDs := o.getUsedChefIdxs(mainRule, seedPos)
				for _, c := range ck {
					if !c.skillOk || usedIDs[rule.Chefs[c.chefIdx].ChefID] {
						continue
					}
					o.simSetChef(mainRule, seedPos, c.chefIdx)
					break
				}

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
	return candidates
}

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

func (o *Optimizer) deepSearchSeed(seed SimState) (int, SimState) {
	o.setSimState(cloneSimState(seed))
	o.bestScore = o.fastCalcScore()
	o.bestSimState = cloneSimState(o.simState)

	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}

	// full climbing
	o.runClimbing()
	if o.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose/deep] after climbing: %d\n", o.bestScore)
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
	if o.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose/deep] after cross-guest: %d\n", o.bestScore)
	}

	// final recipe swap + full climbing
	o.setSimState(cloneSimState(o.bestSimState))
	o.climbRecipeSwap()
	if s := o.fastCalcScore(); s > o.bestScore {
		o.bestScore = s
		o.bestSimState = cloneSimState(o.simState)
	}
	o.runClimbing()

	// joint chef+recipe move as final phase (after cross-guest has settled)
	o.setSimState(cloneSimState(o.bestSimState))
	if o.climbJointChefRecipe() {
		o.runClimbing()
	}

	return o.bestScore, o.bestSimState
}

func (o *Optimizer) clone() *Optimizer {
	c := &Optimizer{
		cfg:                 o.cfg,
		rules:               o.rules, // shared read-only
		gameData:            o.gameData,
		menusByRule:         o.menusByRule,
		ruleMatIndex:        o.ruleMatIndex, // shared read-only
		scratchUsedChefIDs:  make([]bool, len(o.scratchUsedChefIDs)),
		scratchUsedRecIDs:   make([]bool, len(o.scratchUsedRecIDs)),
		scratchUsedRecLocal: make([]bool, len(o.scratchUsedRecLocal)),
	}
	c.initSimState()
	return c
}

func (o *Optimizer) fillEmptySlots() {
	usedChefIDs := make(map[int]bool)
	for ri, rs := range o.simState {
		for _, slot := range rs {
			if slot.ChefIdx >= 0 {
				usedChefIDs[o.rules[ri].Chefs[slot.ChefIdx].ChefID] = true
			}
		}
	}

	type pos struct{ ri, ci int }
	var filled []pos

	for ri := range o.rules {
		rule := &o.rules[ri]
		nc := o.numChefs(ri)
		for ci := 0; ci < nc; ci++ {
			if o.simState[ri][ci].ChefIdx >= 0 {
				continue
			}
			for _, cr := range o.getChefRanking(ri, ci) {
				ch := &rule.Chefs[cr.chefIdx]
				if ch.Got && !usedChefIDs[ch.ChefID] {
					o.simSetChef(ri, ci, cr.chefIdx)
					usedChefIDs[ch.ChefID] = true
					o.greedyFillRecipes(ri, ci)
					filled = append(filled, pos{ri, ci})
					break
				}
			}
		}
	}

	if len(filled) == 0 {
		return
	}

	for _, p := range filled {
		ri, ci := p.ri, p.ci
		rule := &o.rules[ri]
		curChefIdx := o.simState[ri][ci].ChefIdx
		bestSlot := o.simState[ri][ci]
		bestScore := o.fastCalcRuleScore(ri)

		for idx := range rule.Chefs {
			ch := &rule.Chefs[idx]
			if !ch.Got || (usedChefIDs[ch.ChefID] && idx != curChefIdx) {
				continue
			}
			o.simSetChef(ri, ci, idx)
			for reci := 0; reci < 3; reci++ {
				o.simSetRecipe(ri, ci, reci, -1)
			}
			o.greedyFillRecipes(ri, ci)
			score := o.fastCalcRuleScore(ri)
			if score > bestScore {
				bestScore = score
				bestSlot = o.simState[ri][ci]
			}
		}

		if bestSlot.ChefIdx != curChefIdx {
			usedChefIDs[rule.Chefs[curChefIdx].ChefID] = false
			usedChefIDs[rule.Chefs[bestSlot.ChefIdx].ChefID] = true
		}
		o.simState[ri][ci] = bestSlot
		o.markDirty(ri)
	}

	fmt.Fprintf(os.Stderr, "[fill] patched %d empty slot(s)\n", len(filled))
}

func (o *Optimizer) Optimize() (int, SimState, time.Duration) {
	start := time.Now()

	fmt.Fprintf(os.Stderr, "[init] rules=%d\n", len(o.rules))

	// Phase 1: seed generation
	candidates := o.generateSeeds()
	fmt.Fprintf(os.Stderr, "[seed] candidates=%d\n", len(candidates))
	if o.cfg.Verbose {
		for ci, c := range candidates {
			var parts []string
			for ri, rs := range c.state {
				for _, slot := range rs {
					if slot.ChefIdx >= 0 {
						parts = append(parts, fmt.Sprintf("r%d:%s", ri, o.rules[ri].Chefs[slot.ChefIdx].Name))
					}
				}
			}
			fmt.Fprintf(os.Stderr, "[verbose] cand#%d score=%d chefs=[%s]\n", ci, c.score, strings.Join(parts, " "))
		}
	}
	if len(candidates) == 0 {
		return 0, nil, time.Since(start)
	}

	// Phase 2: batch refine ALL candidates in parallel
	activeRules := make([]int, len(o.rules))
	for i := range activeRules {
		activeRules[i] = i
	}
	{
		nRefine := runtime.GOMAXPROCS(0)
		if nRefine > len(candidates) {
			nRefine = len(candidates)
		}
		refineCh := make(chan int, len(candidates))
		for i := range candidates {
			refineCh <- i
		}
		close(refineCh)
		var refineWG sync.WaitGroup
		for w := 0; w < nRefine; w++ {
			refineWG.Add(1)
			go func() {
				defer refineWG.Done()
				worker := o.clone()
				for idx := range refineCh {
					worker.setSimState(candidates[idx].state)
					worker.quickRefine(activeRules, false)
					candidates[idx].score = worker.fastCalcScore()
					candidates[idx].state = cloneSimState(worker.simState)
				}
			}()
		}
		refineWG.Wait()
	}

	partialTopK(candidates, o.cfg.MaxDiverseSeeds, func(a, b seedCandidate) int { return b.score - a.score })
	fmt.Fprintf(os.Stderr, "[refine] best=%d\n", candidates[0].score)

	if o.cfg.Verbose {
		limit := len(candidates)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(os.Stderr, "[verbose] seed#%d score after refine: %d\n", i, candidates[i].score)
		}
	}

	// Phase 3: diversity selection + dedup
	seeds := selectDiverseSeeds(candidates, o.cfg.MaxDiverseSeeds)
	seeds = dedupSeeds(seeds)

	// Aura-chef seeds: refine and append after diversity selection
	auraCands := o.generateAuraSeeds()
	if len(auraCands) > 0 {
		nRefine := runtime.GOMAXPROCS(0)
		if nRefine > len(auraCands) {
			nRefine = len(auraCands)
		}
		refineCh := make(chan int, len(auraCands))
		for i := range auraCands {
			refineCh <- i
		}
		close(refineCh)
		var auraWG sync.WaitGroup
		for w := 0; w < nRefine; w++ {
			auraWG.Add(1)
			go func() {
				defer auraWG.Done()
				worker := o.clone()
				for idx := range refineCh {
					worker.setSimState(auraCands[idx].state)
					worker.quickRefine(activeRules, false)
					auraCands[idx].score = worker.fastCalcScore()
					auraCands[idx].state = cloneSimState(worker.simState)
				}
			}()
		}
		auraWG.Wait()
		slices.SortFunc(auraCands, func(a, b seedCandidate) int { return b.score - a.score })
		limit := 3
		if limit > len(auraCands) {
			limit = len(auraCands)
		}
		for i := 0; i < limit; i++ {
			seeds = append(seeds, cloneSimState(auraCands[i].state))
		}
		seeds = dedupSeeds(seeds)
	}

	// Multi-skill combo seeds: refine and append
	multiCands := o.generateMultiSkillSeeds()
	if len(multiCands) > 0 {
		nRefine := runtime.GOMAXPROCS(0)
		if nRefine > len(multiCands) {
			nRefine = len(multiCands)
		}
		refineCh := make(chan int, len(multiCands))
		for i := range multiCands {
			refineCh <- i
		}
		close(refineCh)
		var multiWG sync.WaitGroup
		for w := 0; w < nRefine; w++ {
			multiWG.Add(1)
			go func() {
				defer multiWG.Done()
				worker := o.clone()
				for idx := range refineCh {
					worker.setSimState(multiCands[idx].state)
					worker.quickRefine(activeRules, false)
					multiCands[idx].score = worker.fastCalcScore()
					multiCands[idx].state = cloneSimState(worker.simState)
				}
			}()
		}
		multiWG.Wait()
		slices.SortFunc(multiCands, func(a, b seedCandidate) int { return b.score - a.score })
		added := 3
		if added > len(multiCands) {
			added = len(multiCands)
		}
		for i := 0; i < added; i++ {
			seeds = append(seeds, cloneSimState(multiCands[i].state))
		}
		seeds = dedupSeeds(seeds)
		fmt.Fprintf(os.Stderr, "[multi-skill] added %d seeds\n", added)
	}

	fmt.Fprintf(os.Stderr, "[diversity] seeds=%d\n", len(seeds))

	if o.cfg.Verbose {
		for si, s := range seeds {
			for ri, rs := range s {
				var names []string
				for _, slot := range rs {
					if slot.ChefIdx >= 0 {
						names = append(names, o.rules[ri].Chefs[slot.ChefIdx].Name)
					}
				}
				fmt.Fprintf(os.Stderr, "[verbose] seed#%d rule%d chefs: %v\n", si, ri, names)
			}
		}
	}

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
		fmt.Fprintf(os.Stderr, "[deep] seed#%d done, score=%d\n", r.idx, r.score)
		if r.score > globalBest {
			globalBest = r.score
			globalBestState = r.state
			winnerIdx = r.idx
		}
	}

	if globalBestState != nil {
		o.setSimState(globalBestState)
		o.fillEmptySlots()
		globalBestState = cloneSimState(o.simState)
		globalBest = o.fastCalcScore()
	}

	elapsed := time.Since(start)
	if o.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] winning seed: #%d with score %d\n", winnerIdx, globalBest)
	}
	fmt.Fprintf(os.Stderr, "[done] best=%d, elapsed=%v\n", globalBest, elapsed)
	return globalBest, globalBestState, elapsed
}

func partialTopK[T any](s []T, k int, cmpFn func(a, b T) int) {
	if k <= 0 || k >= len(s) {
		slices.SortFunc(s, cmpFn)
		return
	}
	quickselect(s, 0, len(s)-1, k, cmpFn)
	slices.SortFunc(s[:k], cmpFn)
}

func quickselect[T any](s []T, lo, hi, k int, cmpFn func(a, b T) int) {
	for lo < hi {
		pivot := s[lo+(hi-lo)/2]
		i, j := lo, hi
		for i <= j {
			for cmpFn(s[i], pivot) < 0 {
				i++
			}
			for cmpFn(pivot, s[j]) < 0 {
				j--
			}
			if i <= j {
				s[i], s[j] = s[j], s[i]
				i++
				j--
			}
		}
		if k <= j-lo+1 {
			hi = j
		} else if k > i-lo {
			k -= i - lo
			lo = i
		} else {
			return
		}
	}
}
