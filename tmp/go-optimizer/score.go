package main

import "math"

func toFixed2(v float64) float64 {
	return math.Round(v*100) / 100
}

func calAddition(base float64, add Addition) float64 {
	return toFixed2((base + add.Abs) * (1 + add.Percent/100))
}

func calReduce(base float64, add Addition) float64 {
	return toFixed2((base - add.Abs) * (1 - add.Percent/100))
}

func setAddition(add *Addition, eff *SkillEffect) {
	if eff.Cal == CalAbs {
		add.Abs = toFixed2(add.Abs + eff.Value)
	} else if eff.Cal == CalPercent {
		add.Percent = toFixed2(add.Percent + eff.Value)
	}
}

func addEffectAddition(add *Addition, eff *SkillEffect, count int) {
	if eff.Cal == CalAbs {
		add.Abs += eff.Value * float64(count)
	} else if eff.Cal == CalPercent {
		add.Percent += eff.Value * float64(count)
	}
}

func getRankInfo(recipe *Recipe, chef *Chef) (rankVal int, rankAddition float64) {
	if recipe == nil || chef == nil || chef.ChefID == 0 {
		return 0, 0
	}
	t := math.MaxFloat64
	if recipe.Stirfry > 0 {
		if v := chef.StirfryVal / float64(recipe.Stirfry); v < t {
			t = v
		}
	}
	if recipe.Boil > 0 {
		if v := chef.BoilVal / float64(recipe.Boil); v < t {
			t = v
		}
	}
	if recipe.Knife > 0 {
		if v := chef.KnifeVal / float64(recipe.Knife); v < t {
			t = v
		}
	}
	if recipe.Fry > 0 {
		if v := chef.FryVal / float64(recipe.Fry); v < t {
			t = v
		}
	}
	if recipe.Bake > 0 {
		if v := chef.BakeVal / float64(recipe.Bake); v < t {
			t = v
		}
	}
	if recipe.Steam > 0 {
		if v := chef.SteamVal / float64(recipe.Steam); v < t {
			t = v
		}
	}
	if t == math.MaxFloat64 {
		return 0, 0
	}
	switch {
	case t >= 5:
		return 5, 100
	case t >= 4:
		return 4, 50
	case t >= 3:
		return 3, 30
	case t >= 2:
		return 2, 10
	case t >= 1:
		return 1, 0
	default:
		return 0, 0
	}
}

func checkSkillCondition(
	eff *SkillEffect,
	chef *Chef,
	chefRecipes *[3]recipeSlot,
	recipe *Recipe,
	recipeQty int,
	allCustomArr []customEntry,
) (bool, int) {
	if eff.ConditionType == CondTypeNone {
		return true, 1
	}

	switch eff.ConditionType {
	case CondTypeRank:
		if recipe != nil && chef != nil && chef.ChefID != 0 {
			rv, _ := getRankInfo(recipe, chef)
			if rv >= eff.ConditionValueInt {
				return true, 1
			}
		}

	case CondTypePerRank:
		count := 0
		if chefRecipes != nil {
			for i := 0; i < 3; i++ {
				if chefRecipes[i].Data != nil {
					rv, _ := getRankInfo(chefRecipes[i].Data, chef)
					if rv >= eff.ConditionValueInt {
						count++
					}
				}
			}
		}
		if count > 0 {
			return true, count
		}

	case CondTypeExcessCookbookNum:
		if recipe != nil && recipeQty >= eff.ConditionValueInt {
			return true, 1
		}

	case CondTypeFewerCookbookNum:
		if recipe != nil && recipeQty <= eff.ConditionValueInt {
			return true, 1
		}

	case CondTypeCookbookRarity:
		if recipe != nil {
			for _, v := range eff.ConditionValueList {
				if recipe.Rarity == v {
					return true, 1
				}
			}
		}

	case CondTypeChefTag:
		if chef != nil && chef.ChefID != 0 {
			for _, tag := range eff.ConditionValueList {
				if tag < len(chef.TagSet) && chef.TagSet[tag] {
					return true, 1
				}
			}
		}

	case CondTypeCookbookTag:
		if recipe != nil {
			count := 0
			for _, tag := range eff.ConditionValueList {
				if tag < len(recipe.TagSet) && recipe.TagSet[tag] {
					count++
				}
			}
			if count > 0 {
				return true, count
			}
		}

	case CondTypeSameSkill:
		if chefRecipes != nil {
			var stirC, boilC, knifeC, fryC, bakeC, steamC int
			d := 0
			for i := 0; i < 3; i++ {
				r := chefRecipes[i].Data
				if r == nil {
					continue
				}
				if r.Stirfry > 0 {
					stirC++
					if stirC == 3 {
						d++
					}
				}
				if r.Boil > 0 {
					boilC++
					if boilC == 3 {
						d++
					}
				}
				if r.Knife > 0 {
					knifeC++
					if knifeC == 3 {
						d++
					}
				}
				if r.Fry > 0 {
					fryC++
					if fryC == 3 {
						d++
					}
				}
				if r.Bake > 0 {
					bakeC++
					if bakeC == 3 {
						d++
					}
				}
				if r.Steam > 0 {
					steamC++
					if steamC == 3 {
						d++
					}
				}
			}
			if d > 0 {
				return true, d
			}
		}

	case CondTypePerSkill:
		count := 0
		cv := eff.ConditionValueInt
		if allCustomArr != nil {
			for _, entry := range allCustomArr {
				for ri := 0; ri < 3; ri++ {
					r := entry.Recipes[ri].Data
					if r == nil {
						continue
					}
					match := false
					switch cv {
					case 1:
						match = r.Stirfry > 0
					case 2:
						match = r.Fry > 0
					case 3:
						match = r.Bake > 0
					case 4:
						match = r.Steam > 0
					case 5:
						match = r.Boil > 0
					case 6:
						match = r.Knife > 0
					}
					if match {
						count++
					}
				}
			}
		}
		if count > 0 {
			return true, count
		}

	case CondTypeMaterialReduce, CondTypeSwordsUnited:
		return true, 1
	}

	return false, 0
}

func isRecipePriceSpecial(eff *SkillEffect, recipe *Recipe, rule *Rule) bool {
	switch eff.Type {
	case EffUseAll:
		return recipe.Rarity == eff.Rarity
	case EffGoldGain:
		return rule == nil || rule.Satiety != 0 || rule.IsActivity
	}
	return false
}

func getRecipeAddition(
	effects []SkillEffect,
	chef *Chef,
	chefRecipes *[3]recipeSlot,
	recipe *Recipe,
	recipeQty int,
	rule *Rule,
) (priceAdd float64, basicAdd Addition) {
	for i := range effects {
		eff := &effects[i]
		pass, count := checkSkillCondition(eff, chef, chefRecipes, recipe, recipeQty, nil)
		if !pass {
			continue
		}
		if recipe.PriceMask&(1<<eff.Type) != 0 || isRecipePriceSpecial(eff, recipe, rule) {
			priceAdd += eff.Value * float64(count)
		}
		if recipe.BasicMask&(1<<eff.Type) != 0 {
			addEffectAddition(&basicAdd, eff, count)
		}
	}
	return
}

func updateEquipmentEffect(equipEffects []SkillEffect, selfUltimate []SkillEffect) []SkillEffect {
	for _, su := range selfUltimate {
		if su.Type == EffMutiEquipmentSkill {
			cloned := make([]SkillEffect, len(equipEffects))
			copy(cloned, equipEffects)
			n := Addition{}
			setAddition(&n, &su)
			for j := range cloned {
				cloned[j].Value = calAddition(cloned[j].Value, n)
			}
			return cloned
		}
	}
	return equipEffects
}

func getMaterialsAddition(recipe *Recipe, materials []Material) float64 {
	t := 0.0
	for _, rm := range recipe.Materials {
		for _, m := range materials {
			if rm.Material == m.MaterialID && m.Addition != 0 {
				t += m.Addition
				break
			}
		}
	}
	return toFixed2(t)
}

func calMaterialReduce(chef *Chef, materialID int, baseQty int) int {
	if chef == nil || len(chef.MaterialEffects) == 0 {
		return baseQty
	}
	add := Addition{}
	for i := range chef.MaterialEffects {
		mr := &chef.MaterialEffects[i]
		for _, mid := range mr.ConditionValueList {
			if mid == materialID {
				if mr.Cal == CalAbs {
					add.Abs = toFixed2(add.Abs + mr.Value)
				} else if mr.Cal == CalPercent {
					add.Percent = toFixed2(add.Percent + mr.Value)
				}
				break
			}
		}
	}
	result := int(math.Ceil(calReduce(float64(baseQty), add)))
	if result < 1 {
		return 1
	}
	return result
}

// materialID→(position+1); 0 = absent. Reuses buf if large enough.
func buildMatIndex(materials []Material, buf []int) []int {
	maxID := 0
	for i := range materials {
		if materials[i].MaterialID > maxID {
			maxID = materials[i].MaterialID
		}
	}
	needed := maxID + 1
	if cap(buf) >= needed {
		buf = buf[:needed]
	} else {
		buf = make([]int, needed)
	}
	clear(buf)
	for i := range materials {
		buf[materials[i].MaterialID] = i + 1
	}
	return buf
}

func getRecipeQuantity(recipe *Recipe, materials []Material, rule *Rule, chef *Chef, matIndex []int) int {
	a := 1
	if !rule.DisableMultiCookbook {
		a = recipe.LimitVal
		if chef != nil {
			for _, le := range chef.MaxLimitEffect {
				if le.Rarity == recipe.Rarity {
					a += le.Value
				}
			}
		}
	}
	for _, rm := range recipe.Materials {
		if rm.Material >= len(matIndex) || matIndex[rm.Material] == 0 {
			return 0
		}
		j := matIndex[rm.Material] - 1
		if materials[j].Quantity > 0 {
			reduced := calMaterialReduce(chef, rm.Material, rm.Quantity)
			c := materials[j].Quantity / reduced
			if c < a {
				a = c
			}
		}
	}
	if a < 0 {
		return 0
	}
	return a
}

func getCustomRound(rule *Rule) int {
	if rule.Satiety != 0 {
		return len(rule.IntentList)
	}
	return 3
}

func isChefAddType(t EffType) bool {
	switch t {
	case EffStirfry, EffBoil, EffKnife, EffFry, EffBake, EffSteam, EffMaxEquipLimit, EffMaterialReduce:
		return true
	}
	return false
}

func getPartialChefAdds(customArr []customEntry, rule *Rule) [][]SkillEffect {
	numChefs := getCustomRound(rule)
	result := make([][]SkillEffect, numChefs)
	getPartialChefAddsInto(customArr, rule, result)
	return result
}

func getPartialChefAddsInto(customArr []customEntry, rule *Rule, result [][]SkillEffect) {
	numChefs := getCustomRound(rule)

	for n, entry := range customArr {
		chef := entry.Chef
		if chef == nil || chef.ChefID == 0 {
			continue
		}
		if chef.ChefID >= len(rule.CalPartialChefSet) || !rule.CalPartialChefSet[chef.ChefID] {
			continue
		}
		for _, eff := range chef.UltimateSkillEffect {
			if eff.Condition == CondScopePartial && isChefAddType(eff.Type) {
				start := 0
				if rule.Satiety != 0 {
					start = n
				}
				for a := start; a < numChefs; a++ {
					result[a] = append(result[a], eff)
				}
			} else if eff.Condition == CondScopeNext && n < numChefs-1 && isChefAddType(eff.Type) {
				result[n+1] = append(result[n+1], eff)
			}
		}
	}
}

func getPartialRecipeAdds(customArr []customEntry, rule *Rule) [][]PartialAdd {
	numChefs := getCustomRound(rule)
	total := 3 * numChefs
	result := make([][]PartialAdd, total)
	getPartialRecipeAddsInto(customArr, rule, result)
	return result
}

func getPartialRecipeAddsInto(customArr []customEntry, rule *Rule, result [][]PartialAdd) {
	numChefs := getCustomRound(rule)
	total := 3 * numChefs

	for n, entry := range customArr {
		chef := entry.Chef
		if chef == nil || chef.ChefID == 0 {
			continue
		}
		if chef.ChefID >= len(rule.CalPartialChefSet) || !rule.CalPartialChefSet[chef.ChefID] {
			continue
		}
		for si := range chef.UltimateSkillEffect {
			eff := &chef.UltimateSkillEffect[si]
			if eff.Condition == CondScopePartial {
				if eff.ConditionType != CondTypeNone && eff.ConditionType != CondTypePerRank && eff.ConditionType != CondTypeSameSkill && eff.ConditionType != CondTypePerSkill {
					for c := range customArr {
						if rule.Satiety != 0 && c < n {
							continue
						}
						m := 3 * c
						for d := 0; d < 3; d++ {
							pass, count := checkSkillCondition(eff, customArr[c].Chef, nil, customArr[c].Recipes[d].Data, customArr[c].Recipes[d].Quantity, nil)
							if pass {
								result[m+d] = append(result[m+d], PartialAdd{Effect: eff, Count: count})
							}
						}
					}
				} else {
					pass, count := checkSkillCondition(eff, chef, entry.Recipes, nil, 0, customArr)
					if pass {
						start := 0
						if rule.Satiety != 0 {
							start = 3 * n
						}
						for a := start; a < total; a++ {
							result[a] = append(result[a], PartialAdd{Effect: eff, Count: count})
						}
					}
				}
			} else if eff.Condition == CondScopeNext && n < numChefs-1 {
				pass, count := checkSkillCondition(eff, chef, entry.Recipes, nil, 0, nil)
				if pass {
					m := 3 * (n + 1)
					for a := 0; a < 3; a++ {
						result[m+a] = append(result[m+a], PartialAdd{Effect: eff, Count: count})
					}
				}
			}
		}
	}
}

func checkIntent(intent *Intent, recipes *[3]recipeSlot, slotIdx int, chef *Chef) bool {
	ct := intent.ConditionType
	if ct == IntCondGroup {
		count := 0
		for i := 0; i < 3; i++ {
			r := recipes[i].Data
			if r != nil && recipeHasSkill(r, intent.CondValSkill) {
				count++
			}
		}
		return count == 3
	}
	if ct == IntCondChefStar {
		return chef != nil && chef.ChefID != 0 && chef.Rarity == intent.ConditionValueInt
	}
	if slotIdx < 0 || slotIdx > 2 {
		return false
	}
	r := recipes[slotIdx].Data
	if r == nil {
		return false
	}
	switch ct {
	case IntCondRank:
		if chef != nil && chef.ChefID != 0 {
			rv, _ := getRankInfo(r, chef)
			return rv >= intent.ConditionValueInt
		}
	case IntCondCondimentSkill:
		return r.Condiment == intent.CondValCondiment
	case IntCondCookSkill:
		return recipeHasSkill(r, intent.CondValSkill)
	case IntCondRarity:
		return r.Rarity == intent.ConditionValueInt
	case IntCondOrder:
		return slotIdx+1 == intent.ConditionValueInt
	case IntCondNone:
		return true
	}
	return false
}

func recipeHasSkill(r *Recipe, skill CookSkill) bool {
	switch skill {
	case CookStirfry:
		return r.Stirfry > 0
	case CookBoil:
		return r.Boil > 0
	case CookKnife:
		return r.Knife > 0
	case CookFry:
		return r.Fry > 0
	case CookBake:
		return r.Bake > 0
	case CookSteam:
		return r.Steam > 0
	}
	return false
}

func getIntentAdds(ruleIndex int, customArr []customEntry, gameData *GameData, rules []Rule) [][]*Intent {
	if ruleIndex < 0 || ruleIndex >= len(rules) {
		return nil
	}
	total := 3 * len(customArr)
	result := make([][]*Intent, total)
	getIntentAddsInto(ruleIndex, customArr, gameData, rules, result)
	return result
}

func getIntentAddsInto(ruleIndex int, customArr []customEntry, gameData *GameData, rules []Rule, result [][]*Intent) {
	if ruleIndex < 0 || ruleIndex >= len(rules) {
		return
	}
	rule := &rules[ruleIndex]
	if rule.Satiety == 0 {
		return
	}

	for _, gbID := range rule.GlobalBuffList {
		buff := gameData.BuffByID[gbID]
		for o := range customArr {
			for p := 0; p < 3; p++ {
				if customArr[o].Recipes[p].Data != nil && checkIntent(buff, customArr[o].Recipes, p, customArr[o].Chef) {
					result[3*o+p] = append(result[3*o+p], buff)
				}
			}
		}
	}

	numRounds := len(rule.IntentList)
	if numRounds > len(customArr) {
		numRounds = len(customArr)
	}
	for c := 0; c < numRounds; c++ {
		for r := 0; r < len(rule.IntentList[c]); r++ {
			intentID := rule.IntentList[c][r]
			intent := gameData.IntentByID[intentID]

			matched := false
			matchedSlot := 0

			if intent.ConditionType == IntCondGroup {
				matched = checkIntent(intent, customArr[c].Recipes, -1, nil)
			} else {
				for o := 0; o < 3; o++ {
					if customArr[c].Recipes[o].Data != nil && checkIntent(intent, customArr[c].Recipes, o, customArr[c].Chef) {
						matched = true
						matchedSlot = o
						break
					}
				}
			}

			if !matched {
				continue
			}
			if intent.EffectType == IntEffCreateBuff {
				buff := gameData.BuffByID[int(intent.EffectValue)]
				for p := 1; p <= buff.LastRounds; p++ {
					if p+c < numRounds {
						for m := 0; m < 3; m++ {
							if checkIntent(buff, customArr[p+c].Recipes, m, customArr[p+c].Chef) {
								result[3*(p+c)+m] = append(result[3*(p+c)+m], buff)
							}
						}
					}
				}
			} else if intent.EffectType == IntEffCreateIntent && matchedSlot < 2 {
				ci2 := gameData.IntentByID[int(intent.EffectValue)]
				if checkIntent(ci2, customArr[c].Recipes, matchedSlot+1, customArr[c].Chef) {
					result[3*c+matchedSlot+1] = append(result[3*c+matchedSlot+1], ci2)
				}
			} else {
				if checkIntent(intent, customArr[c].Recipes, matchedSlot, customArr[c].Chef) {
					result[3*c+matchedSlot] = append(result[3*c+matchedSlot], intent)
				}
			}
		}
	}
}

func getRecipeScore(
	chef *Chef,
	equip *Equip,
	recipe *Recipe,
	quantity int,
	rule *Rule,
	chefRecipes *[3]recipeSlot,
	partialAdds []PartialAdd,
	intentAdds []*Intent,
) (int, int) {
	var rankAddPct float64
	var chefSkillPct float64
	var equipSkillPct float64
	var decorPct float64
	var partialPct float64
	var bonus float64
	var basicAdd Addition

	if chef != nil {
		_, rankAdd := getRankInfo(recipe, chef)

		if !rule.DisableCookbookRank {
			rankAddPct = rankAdd
		}

		if !rule.DisableChefSkillEffect {
			p, b := getRecipeAddition(chef.SpecialSkillEffect, chef, chefRecipes, recipe, quantity, rule)
			chefSkillPct += p
			basicAdd.Abs += b.Abs
			basicAdd.Percent += b.Percent

			p, b = getRecipeAddition(chef.SelfUltimateEffect, chef, chefRecipes, recipe, quantity, rule)
			chefSkillPct += p
			basicAdd.Abs += b.Abs
			basicAdd.Percent += b.Percent

			for i := range partialAdds {
				pa := &partialAdds[i]
				if recipe.PriceMask&(1<<pa.Effect.Type) != 0 || isRecipePriceSpecial(pa.Effect, recipe, rule) {
					partialPct += pa.Effect.Value * float64(pa.Count)
				}
				if recipe.BasicMask&(1<<pa.Effect.Type) != 0 {
					addEffectAddition(&basicAdd, pa.Effect, pa.Count)
				}
			}
		}

		for ai := range chef.Disk.Ambers {
			amber := &chef.Disk.Ambers[ai]
			if amber.Data != nil {
				level := chef.Disk.Level
				if level > 0 && level <= len(amber.Data.AllEffect) {
					p, b := getRecipeAddition(amber.Data.AllEffect[level-1], chef, chefRecipes, recipe, quantity, rule)
					chefSkillPct += p
					basicAdd.Abs += b.Abs
					basicAdd.Percent += b.Percent
				}
			}
		}

		if !rule.DisableEquipSkillEffect && equip != nil && len(equip.Effect) > 0 {
			eqEffects := updateEquipmentEffect(equip.Effect, chef.SelfUltimateEffect)
			p, b := getRecipeAddition(eqEffects, chef, chefRecipes, recipe, quantity, rule)
			equipSkillPct = p
			basicAdd.Abs += b.Abs
			basicAdd.Percent += b.Percent
		}

		bonus += chef.Addition
	}

	sat := recipe.Rarity

	if len(intentAdds) > 0 {
		intentAddPct := 0.0
		for _, ia := range intentAdds {
			if ia.EffectType == IntEffIntentAdd {
				intentAddPct += ia.EffectValue
			}
		}

		var basicChange Addition
		var pctChange float64
		var satChange Addition

		for _, ia := range intentAdds {
			hasBuffID := ia.BuffID != 0
			val := ia.EffectValue
			switch ia.EffectType {
			case IntEffBasicPriceChange:
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				basicChange.Abs += val
			case IntEffBasicPriceChangePercent:
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				basicChange.Percent += val
			case IntEffSatietyChange:
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				satChange.Abs += val
			case IntEffSatietyChangePercent:
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				satChange.Percent += val
			case IntEffSetSatietyValue:
				sat = int(val)
			case IntEffPriceChangePercent:
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				pctChange += val
			}
		}

		basicAdd.Abs += basicChange.Abs
		basicAdd.Percent += basicChange.Percent
		bonus += pctChange / 100
		sat = int(math.Ceil(calAddition(float64(sat), satChange)))
	}

	if !rule.DisableDecorationEffect {
		decorPct = rule.DecorationEffect
	}

	bonus += recipe.Addition

	if rule.MaterialsEffect {
		bonus += getMaterialsAddition(recipe, rule.Materials)
	}

	var recipeRuleBuff float64
	if rule.RecipeEffect != nil {
		if v, ok := rule.RecipeEffect[recipe.RecipeID]; ok {
			recipeRuleBuff = v * 100
		}
	}

	var chefTagBuff float64
	if rule.ChefTagEffect != nil && chef != nil {
		for _, tag := range chef.Tags {
			if v, ok := rule.ChefTagEffect[tag]; ok {
				chefTagBuff += v * 100
			}
		}
	}

	totalSkillPct := (rankAddPct + chefSkillPct + equipSkillPct + decorPct + recipe.UltimateAddition + partialPct + recipeRuleBuff + chefTagBuff) / 100
	adjustedPrice := calAddition(recipe.Price, basicAdd)

	if intentAdds == nil {
		adjustedPrice = math.Floor(adjustedPrice)
	}

	perUnitScore := int(math.Ceil(toFixed2(adjustedPrice * (1 + totalSkillPct + bonus))))
	totalScore := perUnitScore * quantity

	return totalScore, sat
}

func applyChefData(
	chef *Chef,
	equip *Equip,
	globalUltimate []SkillEffect,
	partialAdds []SkillEffect,
	selfUltimateData []SelfUltimateEntry,
	rule *Rule,
	qixia map[int]*QixiaEntry,
) {
	var acc chefDataAccum
	chef.SelfUltimateEffect = nil

	if !rule.DisableChefSkillEffect {
		acc.applySlice(chef, chef.SpecialSkillEffect)
		acc.applySlice(chef, globalUltimate)

		for _, su := range selfUltimateData {
			if su.ChefID == chef.ChefID {
				chef.SelfUltimateEffect = su.Effect
				acc.applySlice(chef, su.Effect)
				break
			}
		}

		for i := range partialAdds {
			pass, _ := checkSkillCondition(&partialAdds[i], chef, nil, nil, 0, nil)
			if pass {
				acc.applyOne(chef, &partialAdds[i])
			}
		}
	}

	if !rule.DisableEquipSkillEffect && equip != nil && len(equip.Effect) > 0 {
		eqEffects := updateEquipmentEffect(equip.Effect, chef.SelfUltimateEffect)
		acc.applySlice(chef, eqEffects)
	}

	for _, amber := range chef.Disk.Ambers {
		if amber.Data != nil {
			level := chef.Disk.Level
			if level > 0 && level <= len(amber.Data.AllEffect) {
				acc.applySlice(chef, amber.Data.AllEffect[level-1])
			}
		}
	}

	if qixia != nil {
		for _, tag := range chef.Tags {
			if q, ok := qixia[tag]; ok {
				acc.stirAdd.Abs += q.Stirfry
				acc.boilAdd.Abs += q.Boil
				acc.knifeAdd.Abs += q.Knife
				acc.fryAdd.Abs += q.Fry
				acc.bakeAdd.Abs += q.Bake
				acc.steamAdd.Abs += q.Steam
			}
		}
	}

	chef.StirfryVal = math.Ceil(calAddition(float64(chef.Stirfry), acc.stirAdd))
	chef.BoilVal = math.Ceil(calAddition(float64(chef.Boil), acc.boilAdd))
	chef.KnifeVal = math.Ceil(calAddition(float64(chef.Knife), acc.knifeAdd))
	chef.FryVal = math.Ceil(calAddition(float64(chef.Fry), acc.fryAdd))
	chef.BakeVal = math.Ceil(calAddition(float64(chef.Bake), acc.bakeAdd))
	chef.SteamVal = math.Ceil(calAddition(float64(chef.Steam), acc.steamAdd))

	if acc.maxEquipN > 0 {
		lim := make([]LimitEffect, acc.maxEquipN)
		copy(lim, acc.maxEquipLimits[:acc.maxEquipN])
		chef.MaxLimitEffect = lim
	} else {
		chef.MaxLimitEffect = nil
	}
	if acc.matReduceN > 0 {
		mr := make([]MaterialReduce, acc.matReduceN)
		copy(mr, acc.matReduces[:acc.matReduceN])
		chef.MaterialEffects = mr
	} else {
		chef.MaterialEffects = nil
	}
}

// Avoids heap-allocating a []SkillEffect per applyChefData call.
type chefDataAccum struct {
	stirAdd, boilAdd, knifeAdd, fryAdd, bakeAdd, steamAdd Addition
	maxEquipLimits [8]LimitEffect
	maxEquipN      int
	matReduces     [8]MaterialReduce
	matReduceN     int
}

func (a *chefDataAccum) applySlice(chef *Chef, effects []SkillEffect) {
	for i := range effects {
		a.applyOne(chef, &effects[i])
	}
}

func (a *chefDataAccum) applyOne(chef *Chef, eff *SkillEffect) {
	if eff.Tag != 0 && (eff.Tag >= len(chef.TagSet) || !chef.TagSet[eff.Tag]) {
		return
	}
	switch eff.Type {
	case EffStirfry:
		setAddition(&a.stirAdd, eff)
	case EffBoil:
		setAddition(&a.boilAdd, eff)
	case EffKnife:
		setAddition(&a.knifeAdd, eff)
	case EffFry:
		setAddition(&a.fryAdd, eff)
	case EffBake:
		setAddition(&a.bakeAdd, eff)
	case EffSteam:
		setAddition(&a.steamAdd, eff)
	case EffMaxEquipLimit:
		if eff.Condition == CondScopeSelf || eff.Condition == CondScopePartial {
			if a.maxEquipN < len(a.maxEquipLimits) {
				a.maxEquipLimits[a.maxEquipN] = LimitEffect{Rarity: eff.Rarity, Value: int(eff.Value)}
				a.maxEquipN++
			}
		}
	case EffMaterialReduce:
		if eff.Condition == CondScopeSelf || eff.Condition == CondScopePartial {
			if a.matReduceN < len(a.matReduces) {
				a.matReduces[a.matReduceN] = MaterialReduce{
					ConditionValueList: eff.ConditionValueList,
					Cal:                eff.Cal,
					Value:              eff.Value,
				}
				a.matReduceN++
			}
		}
	}
}

type resolvedSlot struct {
	chefObj  *Chef
	equipObj *Equip
	recipes  [3]recipeSlot
}

func resolveRuleState(rule *Rule, rs RuleState) []resolvedSlot {
	slots := make([]resolvedSlot, len(rs))
	resolveRuleStateInto(rule, rs, slots, nil)
	return slots
}

// If chefBuf is non-nil and large enough, chef clones go there to avoid heap allocation.
func resolveRuleStateInto(rule *Rule, rs RuleState, slots []resolvedSlot, chefBuf []Chef) {
	for ci := range rs {
		ss := &rs[ci]
		slots[ci] = resolvedSlot{}
		if ss.ChefIdx >= 0 && ss.ChefIdx < len(rule.Chefs) {
			if chefBuf != nil {
				chefBuf[ci] = rule.Chefs[ss.ChefIdx]
				chefBuf[ci].SelfUltimateEffect = nil
				chefBuf[ci].MaxLimitEffect = nil
				chefBuf[ci].MaterialEffects = nil
				slots[ci].chefObj = &chefBuf[ci]
			} else {
				clone := rule.Chefs[ss.ChefIdx]
				clone.SelfUltimateEffect = nil
				clone.MaxLimitEffect = nil
				clone.MaterialEffects = nil
				slots[ci].chefObj = &clone
			}
			slots[ci].equipObj = rule.Chefs[ss.ChefIdx].EquipEffect
		}

		for reci := 0; reci < 3; reci++ {
			ri := ss.RecipeIdxs[reci]
			if ri >= 0 && ri < len(rule.Recipes) {
				slots[ci].recipes[reci] = recipeSlot{
					Data:     &rule.Recipes[ri],
					Quantity: ss.Quantities[reci],
					Max:      ss.MaxQty[reci],
				}
			}
		}
	}
}

func applyChefDataForRule(rule *Rule, slots []resolvedSlot, partialChefAdds [][]SkillEffect) {
	for ci := range slots {
		chef := slots[ci].chefObj
		if chef != nil && chef.ChefID != 0 {
			var pa []SkillEffect
			if ci < len(partialChefAdds) {
				pa = partialChefAdds[ci]
			}
			applyChefData(
				chef,
				slots[ci].equipObj,
				rule.CalGlobalUltimateData,
				pa,
				rule.CalSelfUltimateData,
				rule,
				rule.CalQixiaData,
			)
		}
	}
}

func calcRuleScore(rules []Rule, simState SimState, ruleIndex int, gameData *GameData) int {
	var sc ScratchBuffers
	return calcRuleScoreReuse(rules, simState, ruleIndex, gameData, &sc)
}

type ScratchBuffers struct {
	materials       []Material
	slots           []resolvedSlot
	customArr       []customEntry
	intentAdds      [][]*Intent
	partialAdds     [][]PartialAdd
	partialChefAdds [][]SkillEffect
	chefBuf         []Chef
	matIndex        []int // indexed by MaterialID, 0 = absent, positive = position+1
}

func calcRuleScoreReuse(rules []Rule, simState SimState, ruleIndex int, gameData *GameData, sc *ScratchBuffers) int {
	if ruleIndex < 0 || ruleIndex >= len(rules) || ruleIndex >= len(simState) {
		return 0
	}
	rule := &rules[ruleIndex]
	rs := simState[ruleIndex]
	nc := len(rs)

	if cap(sc.slots) < nc {
		sc.slots = make([]resolvedSlot, nc)
	}
	sc.slots = sc.slots[:nc]
	if cap(sc.chefBuf) < nc {
		sc.chefBuf = make([]Chef, nc)
	}
	sc.chefBuf = sc.chefBuf[:nc]
	resolveRuleStateInto(rule, rs, sc.slots, sc.chefBuf)

	if cap(sc.customArr) < nc {
		sc.customArr = make([]customEntry, nc)
	}
	sc.customArr = sc.customArr[:nc]
	buildCustomArrInto(sc.slots, sc.customArr)

	numChefs := getCustomRound(rule)
	if cap(sc.partialChefAdds) < numChefs {
		sc.partialChefAdds = make([][]SkillEffect, numChefs)
	}
	sc.partialChefAdds = sc.partialChefAdds[:numChefs]
	for i := range sc.partialChefAdds {
		sc.partialChefAdds[i] = sc.partialChefAdds[i][:0]
	}
	getPartialChefAddsInto(sc.customArr, rule, sc.partialChefAdds)

	applyChefDataForRule(rule, sc.slots, sc.partialChefAdds)

	nm := len(rule.Materials)
	if cap(sc.materials) < nm {
		sc.materials = make([]Material, nm)
	}
	sc.materials = sc.materials[:nm]
	copy(sc.materials, rule.Materials)

	sc.matIndex = buildMatIndex(sc.materials, sc.matIndex)

	for ci := range sc.slots {
		for reci := 0; reci < 3; reci++ {
			rec := &sc.slots[ci].recipes[reci]
			if rec.Data != nil {
				recipeMax := getRecipeQuantity(rec.Data, sc.materials, rule, sc.slots[ci].chefObj, sc.matIndex)
				if rule.DisableMultiCookbook && recipeMax > 1 {
					recipeMax = 1
				}
				rec.Max = recipeMax
				if rec.Quantity > recipeMax {
					rec.Quantity = recipeMax
				}
				updateMaterialsData(sc.materials, rec.Data, rec.Quantity, sc.slots[ci].chefObj, sc.matIndex)
			}
		}
	}

	if cap(sc.customArr) < nc {
		sc.customArr = make([]customEntry, nc)
	}
	sc.customArr = sc.customArr[:nc]
	buildCustomArrInto(sc.slots, sc.customArr)

	total := 3 * nc

	if cap(sc.intentAdds) < total {
		sc.intentAdds = make([][]*Intent, total)
	}
	sc.intentAdds = sc.intentAdds[:total]
	for i := range sc.intentAdds {
		sc.intentAdds[i] = sc.intentAdds[i][:0]
	}
	getIntentAddsInto(ruleIndex, sc.customArr, gameData, rules, sc.intentAdds)

	if cap(sc.partialAdds) < total {
		sc.partialAdds = make([][]PartialAdd, total)
	}
	sc.partialAdds = sc.partialAdds[:total]
	for i := range sc.partialAdds {
		sc.partialAdds[i] = sc.partialAdds[i][:0]
	}
	getPartialRecipeAddsInto(sc.customArr, rule, sc.partialAdds)

	rawScore := 0
	satTotal := 0
	recipeCount := 0
	for ci := range sc.slots {
		for reci := 0; reci < 3; reci++ {
			rec := &sc.slots[ci].recipes[reci]
			if rec.Data != nil {
				var pa []PartialAdd
				idx := 3*ci + reci
				if idx < len(sc.partialAdds) {
					pa = sc.partialAdds[idx]
				}
				var ia []*Intent
				if idx < len(sc.intentAdds) {
					ia = sc.intentAdds[idx]
				}

				totalScore, sat := getRecipeScore(
					sc.slots[ci].chefObj,
					sc.slots[ci].equipObj,
					rec.Data,
					rec.Quantity,
					rule,
					&sc.slots[ci].recipes,
					pa,
					ia,
				)

				actAdd := rec.Data.ActivityAddition
				rawScore += int(math.Ceil(toFixed2(float64(totalScore) * (1 + actAdd/100))))
				rec.Satiety = sat
				satTotal += sat
				recipeCount++
			}
		}
	}

	return applyRuleModifiers(rawScore, satTotal, recipeCount, rule)
}

// Shared tail of CalcRuleDetail and calcRuleScoreReuse.
func applyRuleModifiers(rawScore, satTotal, recipeCount int, rule *Rule) int {
	scoreMultiply := 1.0
	if rule.ScoreMultiply != 0 {
		scoreMultiply = rule.ScoreMultiply
	}
	scorePow := 1.0
	if rule.ScorePow != 0 {
		scorePow = rule.ScorePow
	}

	result := rawScore
	modified := toFixed2(math.Pow(float64(result), scorePow) * scoreMultiply)
	if rule.IsActivity {
		result = int(math.Ceil(modified))
	} else {
		result = int(math.Floor(modified))
	}
	if result != 0 && rule.ScoreAdd != 0 {
		result += rule.ScoreAdd
	}

	if rule.Satiety != 0 {
		expected := 3 * len(rule.IntentList)
		if recipeCount == expected {
			satAdd := getSatietyPercent(satTotal, rule)
			result = calSatietyAdd(result, satAdd)
		}
	}
	return result
}

func getSatietyPercent(satTotal int, rule *Rule) float64 {
	if rule == nil {
		return 0
	}
	if rule.SatisfyRewardType == 1 && satTotal == rule.Satiety {
		return rule.SatisfyExtraValue
	}
	return -rule.SatisfyDeductValue * math.Abs(float64(satTotal-rule.Satiety))
}

func calSatietyAdd(score int, satAdd float64) int {
	return int(math.Ceil(toFixed2(float64(score) * (1 + 0.01*satAdd))))
}

func buildCustomArr(slots []resolvedSlot) []customEntry {
	arr := make([]customEntry, len(slots))
	buildCustomArrInto(slots, arr)
	return arr
}

func buildCustomArrInto(slots []resolvedSlot, arr []customEntry) {
	for ci := range slots {
		arr[ci] = customEntry{
			Chef:    slots[ci].chefObj,
			Equip:   slots[ci].equipObj,
			Recipes: &slots[ci].recipes,
		}
	}
}

func cloneMaterials(mats []Material) []Material {
	c := make([]Material, len(mats))
	copy(c, mats)
	return c
}

// matIndex: []int indexed by MaterialID, 0 = absent, positive = position+1.
func updateMaterialsData(materials []Material, recipe *Recipe, quantity int, chef *Chef, matIndex []int) {
	if recipe == nil {
		return
	}
	for _, rm := range recipe.Materials {
		if rm.Material < len(matIndex) {
			if v := matIndex[rm.Material]; v > 0 && materials[v-1].Quantity > 0 {
				reduced := calMaterialReduce(chef, rm.Material, rm.Quantity)
				materials[v-1].Quantity -= reduced * quantity
			}
		}
	}
}
