package main

import (
	"math"
	"strconv"
	"strings"
)

// ── toFixed(2) equivalent ──

func toFixed2(v float64) float64 {
	return math.Round(v*100) / 100
}

// ── Addition helpers ──

// calAddition: JS line 363 — +((i + e.abs) * (1 + e.percent / 100)).toFixed(2)
func calAddition(base float64, add Addition) float64 {
	return toFixed2((base + add.Abs) * (1 + add.Percent/100))
}

// calReduce: JS line 199
func calReduce(base float64, add Addition) float64 {
	return toFixed2((base - add.Abs) * (1 - add.Percent/100))
}

func setAddition(add *Addition, eff *SkillEffect) {
	if eff.Cal == "Abs" {
		add.Abs = toFixed2(add.Abs + eff.Value)
	} else if eff.Cal == "Percent" {
		add.Percent = toFixed2(add.Percent + eff.Value)
	}
}

func addEffectAddition(add *Addition, eff *SkillEffect, count int) {
	if eff.Cal == "Abs" {
		add.Abs += eff.Value * float64(count)
	} else if eff.Cal == "Percent" {
		add.Percent += eff.Value * float64(count)
	}
}

// ── GetRankInfo: JS line 1 ──

// GetRankInfo returns the rank level (0-5) and corresponding addition percentage for a chef cooking a recipe.
func GetRankInfo(recipe *Recipe, chef *Chef) (rankVal int, rankAddition float64) {
	if recipe == nil || chef == nil || chef.ChefID == 0 {
		return 0, 0
	}
	t := math.MaxFloat64
	if recipe.Stirfry > 0 {
		t = math.Min(t, chef.StirfryVal/float64(recipe.Stirfry))
	}
	if recipe.Boil > 0 {
		t = math.Min(t, chef.BoilVal/float64(recipe.Boil))
	}
	if recipe.Knife > 0 {
		t = math.Min(t, chef.KnifeVal/float64(recipe.Knife))
	}
	if recipe.Fry > 0 {
		t = math.Min(t, chef.FryVal/float64(recipe.Fry))
	}
	if recipe.Bake > 0 {
		t = math.Min(t, chef.BakeVal/float64(recipe.Bake))
	}
	if recipe.Steam > 0 {
		t = math.Min(t, chef.SteamVal/float64(recipe.Steam))
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

// ── checkSkillCondition: JS line 679 ──
// Returns (pass, count).

func checkSkillCondition(
	eff *SkillEffect,
	chef *Chef,
	chefRecipes *[3]recipeSlot,
	recipe *Recipe,
	recipeQty int,
	allCustomArr []customEntry,
) (bool, int) {
	if eff == nil {
		return false, 0
	}
	if eff.ConditionType == "" {
		return true, 1
	}

	switch eff.ConditionType {
	case "Rank":
		if recipe != nil && chef != nil && chef.ChefID != 0 {
			rv, _ := GetRankInfo(recipe, chef)
			if rv >= conditionValueInt(eff.ConditionValue) {
				return true, 1
			}
		}

	case "PerRank":
		count := 0
		if chefRecipes != nil {
			for i := 0; i < 3; i++ {
				if chefRecipes[i].Data != nil {
					rv, _ := GetRankInfo(chefRecipes[i].Data, chef)
					if rv >= conditionValueInt(eff.ConditionValue) {
						count++
					}
				}
			}
		}
		if count > 0 {
			return true, count
		}

	case "ExcessCookbookNum":
		if recipe != nil && recipeQty >= conditionValueInt(eff.ConditionValue) {
			return true, 1
		}

	case "FewerCookbookNum":
		if recipe != nil && recipeQty <= conditionValueInt(eff.ConditionValue) {
			return true, 1
		}

	case "CookbookRarity":
		if recipe != nil {
			for _, v := range eff.ConditionValueList {
				if recipe.Rarity == interfaceToInt(v) {
					return true, 1
				}
			}
		}

	case "ChefTag":
		if chef != nil && chef.ChefID != 0 {
			count := 0
			for _, v := range eff.ConditionValueList {
				tag := interfaceToInt(v)
				for _, ct := range chef.Tags {
					if ct == tag {
						count++
						break
					}
				}
			}
			if count > 0 {
				return true, 1
			}
		}

	case "CookbookTag":
		if recipe != nil {
			count := 0
			for _, v := range eff.ConditionValueList {
				tag := interfaceToInt(v)
				for _, rt := range recipe.Tags {
					if rt == tag {
						count++
						break
					}
				}
			}
			if count > 0 {
				return true, count
			}
		}

	case "SameSkill":
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

	case "PerSkill":
		count := 0
		cv := conditionValueInt(eff.ConditionValue)
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

	case "MaterialReduce", "SwordsUnited":
		return true, 1
	}

	return false, 0
}

// ── isRecipePriceAddition: JS line 43 ──

func isRecipePriceAddition(eff *SkillEffect, recipe *Recipe, rule *Rule) bool {
	if eff == nil || recipe == nil {
		return false
	}
	switch eff.Type {
	case "UseAll":
		return recipe.Rarity == eff.Rarity
	case "UseFish":
		return recipeHasMaterialOrigin(recipe, "池塘")
	case "UseCreation":
		return recipeHasMaterialOrigin(recipe, "作坊")
	case "UseMeat":
		return recipeHasMaterialOriginAny(recipe, "牧场", "鸡舍", "猪圈")
	case "UseVegetable":
		return recipeHasMaterialOriginAny(recipe, "菜棚", "菜地", "森林")
	case "UseStirfry":
		return recipe.Stirfry > 0
	case "UseBoil":
		return recipe.Boil > 0
	case "UseFry":
		return recipe.Fry > 0
	case "UseKnife":
		return recipe.Knife > 0
	case "UseBake":
		return recipe.Bake > 0
	case "UseSteam":
		return recipe.Steam > 0
	case "UseSweet":
		return recipe.Condiment == "Sweet"
	case "UseSour":
		return recipe.Condiment == "Sour"
	case "UseSpicy":
		return recipe.Condiment == "Spicy"
	case "UseSalty":
		return recipe.Condiment == "Salty"
	case "UseBitter":
		return recipe.Condiment == "Bitter"
	case "UseTasty":
		return recipe.Condiment == "Tasty"
	case "Gold_Gain":
		if rule == nil || rule.Satiety != 0 || rule.IsActivity {
			return true
		}
	case "CookbookPrice":
		return true
	}
	return false
}

// ── isRecipeBasicAddition: JS line 107 ──

func isRecipeBasicAddition(eff *SkillEffect, recipe *Recipe) bool {
	if eff == nil || recipe == nil {
		return false
	}
	switch eff.Type {
	case "BasicPrice":
		return true
	case "BasicPriceUseFish":
		return recipeHasMaterialOrigin(recipe, "池塘")
	case "BasicPriceUseCreation":
		return recipeHasMaterialOrigin(recipe, "作坊")
	case "BasicPriceUseMeat":
		return recipeHasMaterialOriginAny(recipe, "牧场", "鸡舍", "猪圈")
	case "BasicPriceUseVegetable":
		return recipeHasMaterialOriginAny(recipe, "菜棚", "菜地", "森林")
	case "BasicPriceUseStirfry":
		return recipe.Stirfry > 0
	case "BasicPriceUseBoil":
		return recipe.Boil > 0
	case "BasicPriceUseFry":
		return recipe.Fry > 0
	case "BasicPriceUseKnife":
		return recipe.Knife > 0
	case "BasicPriceUseBake":
		return recipe.Bake > 0
	case "BasicPriceUseSteam":
		return recipe.Steam > 0
	case "BasicPriceUseSweet":
		return recipe.Condiment == "Sweet"
	case "BasicPriceUseSour":
		return recipe.Condiment == "Sour"
	case "BasicPriceUseSpicy":
		return recipe.Condiment == "Spicy"
	case "BasicPriceUseSalty":
		return recipe.Condiment == "Salty"
	case "BasicPriceUseBitter":
		return recipe.Condiment == "Bitter"
	case "BasicPriceUseTasty":
		return recipe.Condiment == "Tasty"
	}
	return false
}

func recipeHasMaterialOrigin(recipe *Recipe, origin string) bool {
	if recipe == nil {
		return false
	}
	for i := range recipe.Materials {
		if recipe.Materials[i].Origin == origin {
			return true
		}
	}
	return false
}

func recipeHasMaterialOriginAny(recipe *Recipe, origins ...string) bool {
	if recipe == nil {
		return false
	}
	for i := range recipe.Materials {
		for _, o := range origins {
			if recipe.Materials[i].Origin == o {
				return true
			}
		}
	}
	return false
}

// ── getRecipeAddition: JS line 29 ──

func getRecipeAddition(
	effects []SkillEffect,
	chef *Chef,
	chefRecipes *[3]recipeSlot,
	recipe *Recipe,
	recipeQty int,
	rule *Rule,
) (priceAdd float64, basicAdd Addition) {
	if recipe == nil {
		return
	}
	for i := range effects {
		eff := &effects[i]
		pass, count := checkSkillCondition(eff, chef, chefRecipes, recipe, recipeQty, nil)
		if !pass {
			continue
		}
		if isRecipePriceAddition(eff, recipe, rule) {
			priceAdd += eff.Value * float64(count)
		}
		if isRecipeBasicAddition(eff, recipe) {
			addEffectAddition(&basicAdd, eff, count)
		}
	}
	return
}

// ── updateEquipmentEffect: JS line 372 ──

func updateEquipmentEffect(equipEffects []SkillEffect, selfUltimate []SkillEffect) []SkillEffect {
	for _, su := range selfUltimate {
		if su.Type == "MutiEquipmentSkill" {
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

// ── getMaterialsAddition: JS line 164 ──

func getMaterialsAddition(recipe *Recipe, materials []Material) float64 {
	if recipe == nil {
		return 0
	}
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

// ── calMaterialReduce: JS line 187 ──

func calMaterialReduce(chef *Chef, materialID int, baseQty int) int {
	if chef == nil || len(chef.MaterialEffects) == 0 {
		return baseQty
	}
	add := Addition{}
	for i := range chef.MaterialEffects {
		mr := &chef.MaterialEffects[i]
		if mr.ConditionType == "MaterialReduce" {
			for _, mid := range mr.ConditionValueList {
				if mid == materialID {
					if mr.Cal == "Abs" {
						add.Abs = toFixed2(add.Abs + mr.Value)
					} else if mr.Cal == "Percent" {
						add.Percent = toFixed2(add.Percent + mr.Value)
					}
					break
				}
			}
		}
	}
	result := int(math.Ceil(calReduce(float64(baseQty), add)))
	if result < 1 {
		return 1
	}
	return result
}

// ── GetRecipeQuantity: JS line 202 ──

// GetRecipeQuantity computes the maximum number of times a recipe can be cooked given available materials.
func GetRecipeQuantity(recipe *Recipe, materials []Material, rule *Rule, chef *Chef) int {
	if recipe == nil || rule == nil {
		return 0
	}
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
		found := false
		for _, m := range materials {
			if rm.Material == m.MaterialID {
				found = true
				if m.Quantity > 0 {
					reduced := calMaterialReduce(chef, rm.Material, rm.Quantity)
					c := m.Quantity / reduced
					if c < a {
						a = c
					}
				}
				break
			}
		}
		if !found {
			return 0
		}
	}
	if a < 0 {
		return 0
	}
	return a
}

// ── getCustomRound: JS line 624 ──

func getCustomRound(rule *Rule) int {
	if rule.Satiety != 0 {
		return len(rule.IntentList)
	}
	return 3
}

// ── isChefAddType: JS line 621 ──

func isChefAddType(t string) bool {
	switch t {
	case "Stirfry", "Boil", "Knife", "Fry", "Bake", "Steam", "MaxEquipLimit", "MaterialReduce":
		return true
	}
	return false
}

// ── getPartialChefAdds: JS line 808 ──

func getPartialChefAdds(customArr []customEntry, rule *Rule) [][]SkillEffect {
	numChefs := getCustomRound(rule)
	result := make([][]SkillEffect, numChefs)

	for n, entry := range customArr {
		chef := entry.Chef
		if chef == nil || chef.ChefID == 0 {
			continue
		}
		if !intSliceContains(rule.CalPartialChefIDs, chef.ChefID) {
			continue
		}
		for _, eff := range chef.UltimateSkillEffect {
			if eff.Condition == "Partial" && isChefAddType(eff.Type) {
				start := 0
				if rule.Satiety != 0 {
					start = n
				}
				for a := start; a < numChefs; a++ {
					result[a] = append(result[a], eff)
				}
			} else if eff.Condition == "Next" && n < numChefs-1 && isChefAddType(eff.Type) {
				result[n+1] = append(result[n+1], eff)
			}
		}
	}
	return result
}

// ── getPartialRecipeAdds: JS line 627 ──

func getPartialRecipeAdds(customArr []customEntry, rule *Rule) [][]PartialAdd {
	numChefs := getCustomRound(rule)
	total := 3 * numChefs
	result := make([][]PartialAdd, total)

	for n, entry := range customArr {
		chef := entry.Chef
		if chef == nil || chef.ChefID == 0 {
			continue
		}
		if !intSliceContains(rule.CalPartialChefIDs, chef.ChefID) {
			continue
		}
		for si := range chef.UltimateSkillEffect {
			eff := &chef.UltimateSkillEffect[si]
			if eff.Condition == "Partial" {
				if eff.ConditionType != "" && eff.ConditionType != "PerRank" && eff.ConditionType != "SameSkill" && eff.ConditionType != "PerSkill" {
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
			} else if eff.Condition == "Next" && n < numChefs-1 {
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
	return result
}

// ── checkIntent: JS food.min.js line 7176 ──

func checkIntent(intent *Intent, recipes *[3]recipeSlot, slotIdx int, chef *Chef) bool {
	if intent == nil || recipes == nil {
		return false
	}
	ct := intent.ConditionType
	if ct == "Group" {
		count := 0
		skillName := strings.ToLower(interfaceToString(intent.ConditionValue))
		for i := 0; i < 3; i++ {
			r := recipes[i].Data
			if r != nil && recipeHasSkill(r, skillName) {
				count++
			}
		}
		return count == 3
	}
	if ct == "ChefStar" {
		return chef != nil && chef.ChefID != 0 && chef.Rarity == conditionValueInt(intent.ConditionValue)
	}
	if slotIdx < 0 || slotIdx > 2 {
		return false
	}
	r := recipes[slotIdx].Data
	if r == nil {
		return false
	}
	switch ct {
	case "Rank":
		if chef != nil && chef.ChefID != 0 {
			rv, _ := GetRankInfo(r, chef)
			return rv >= conditionValueInt(intent.ConditionValue)
		}
	case "CondimentSkill":
		return r.Condiment == interfaceToString(intent.ConditionValue)
	case "CookSkill":
		return recipeHasSkill(r, strings.ToLower(interfaceToString(intent.ConditionValue)))
	case "Rarity":
		return r.Rarity == conditionValueInt(intent.ConditionValue)
	case "Order":
		return slotIdx+1 == conditionValueInt(intent.ConditionValue)
	case "":
		return true
	}
	return false
}

func recipeHasSkill(r *Recipe, skill string) bool {
	switch skill {
	case "stirfry":
		return r.Stirfry > 0
	case "boil":
		return r.Boil > 0
	case "knife":
		return r.Knife > 0
	case "fry":
		return r.Fry > 0
	case "bake":
		return r.Bake > 0
	case "steam":
		return r.Steam > 0
	}
	return false
}

// ── getIntentAdds: JS food.min.js line 7121 ──

func getIntentAdds(ruleIndex int, customArr []customEntry, gameData *GameData, rules []Rule) [][]Intent {
	if ruleIndex < 0 || ruleIndex >= len(rules) {
		return nil
	}
	rule := &rules[ruleIndex]
	total := 3 * len(customArr)
	result := make([][]Intent, total)

	if rule.Satiety == 0 {
		return result
	}

	// Global buffs
	if len(rule.GlobalBuffList) > 0 && gameData != nil {
		for _, gbID := range rule.GlobalBuffList {
			for bi := range gameData.Buffs {
				buff := &gameData.Buffs[bi]
				if gbID == buff.BuffID {
					for o := range customArr {
						for p := 0; p < 3; p++ {
							if customArr[o].Recipes[p].Data != nil && checkIntent(buff, customArr[o].Recipes, p, customArr[o].Chef) {
								result[3*o+p] = append(result[3*o+p], *buff)
							}
						}
					}
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
			if gameData == nil {
				continue
			}
			for ni := range gameData.Intents {
				intent := &gameData.Intents[ni]
				if intentID != intent.IntentID {
					continue
				}

				matched := false
				matchedSlot := 0

				if intent.ConditionType == "Group" {
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

				if matched {
					if intent.EffectType == "CreateBuff" {
						buffID := int(intent.EffectValue)
						for bi := range gameData.Buffs {
							buff := &gameData.Buffs[bi]
							if buff.BuffID == buffID {
								for p := 1; p <= buff.LastRounds; p++ {
									if p+c < numRounds {
										for m := 0; m < 3; m++ {
											if checkIntent(buff, customArr[p+c].Recipes, m, customArr[p+c].Chef) {
												result[3*(p+c)+m] = append(result[3*(p+c)+m], *buff)
											}
										}
									}
								}
								break
							}
						}
					} else if intent.EffectType == "CreateIntent" && matchedSlot < 2 {
						createdID := int(intent.EffectValue)
						for ci := range gameData.Intents {
							ci2 := &gameData.Intents[ci]
							if ci2.IntentID == createdID {
								if checkIntent(ci2, customArr[c].Recipes, matchedSlot+1, customArr[c].Chef) {
									result[3*c+matchedSlot+1] = append(result[3*c+matchedSlot+1], *ci2)
								}
								break
							}
						}
					} else {
						if checkIntent(intent, customArr[c].Recipes, matchedSlot, customArr[c].Chef) {
							result[3*c+matchedSlot] = append(result[3*c+matchedSlot], *intent)
						}
					}
				}
				break
			}
		}
	}

	return result
}

// ── getRecipeScore: ports getRecipeResult (JS line 225), returns (totalScore, satiety) ──

func getRecipeScore(
	chef *Chef,
	equip *Equip,
	recipe *Recipe,
	quantity int,
	maxQty int,
	rule *Rule,
	chefRecipes *[3]recipeSlot,
	partialAdds []PartialAdd,
	intentAdds []Intent,
) (int, int) {
	if recipe == nil || rule == nil {
		return 0, 0
	}

	var rankAddPct float64
	var chefSkillPct float64
	var equipSkillPct float64
	var decorPct float64
	var partialPct float64
	var bonus float64
	var h Addition

	if chef != nil && chef.ChefID != 0 {
		_, rankAdd := GetRankInfo(recipe, chef)

		if !rule.DisableCookbookRank {
			rankAddPct = rankAdd
		}

		if !rule.DisableChefSkillEffect {
			p, b := getRecipeAddition(chef.SpecialSkillEffect, chef, chefRecipes, recipe, quantity, rule)
			chefSkillPct += p
			h.Abs += b.Abs
			h.Percent += b.Percent

			p, b = getRecipeAddition(chef.SelfUltimateEffect, chef, chefRecipes, recipe, quantity, rule)
			chefSkillPct += p
			h.Abs += b.Abs
			h.Percent += b.Percent

			for i := range partialAdds {
				pa := &partialAdds[i]
				if isRecipePriceAddition(pa.Effect, recipe, rule) {
					partialPct += pa.Effect.Value * float64(pa.Count)
				}
				if isRecipeBasicAddition(pa.Effect, recipe) {
					addEffectAddition(&h, pa.Effect, pa.Count)
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
					h.Abs += b.Abs
					h.Percent += b.Percent
				}
			}
		}

		if !rule.DisableEquipSkillEffect && equip != nil && len(equip.Effect) > 0 {
			eqEffects := updateEquipmentEffect(equip.Effect, chef.SelfUltimateEffect)
			p, b := getRecipeAddition(eqEffects, chef, chefRecipes, recipe, quantity, rule)
			equipSkillPct = p
			h.Abs += b.Abs
			h.Percent += b.Percent
		}

		bonus += chef.Addition
	}

	sat := recipe.Rarity

	// Intent adds
	if len(intentAdds) > 0 {
		intentAddPct := 0.0
		for _, ia := range intentAdds {
			if ia.EffectType == "IntentAdd" {
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
			case "BasicPriceChange":
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				basicChange.Abs += val
			case "BasicPriceChangePercent":
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				basicChange.Percent += val
			case "SatietyChange":
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				satChange.Abs += val
			case "SatietyChangePercent":
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				satChange.Percent += val
			case "SetSatietyValue":
				sat = int(val)
			case "PriceChangePercent":
				if !hasBuffID {
					val *= 1 + 0.01*intentAddPct
				}
				pctChange += val
			}
		}

		h.Abs += basicChange.Abs
		h.Percent += basicChange.Percent
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

	// JS lines 339-353
	L := (rankAddPct + chefSkillPct + equipSkillPct + decorPct + recipe.UltimateAddition + partialPct + recipeRuleBuff + chefTagBuff) / 100
	J := calAddition(recipe.Price, h)

	if intentAdds == nil {
		J = math.Floor(J)
	}

	K := int(math.Ceil(toFixed2(J * (1 + L + bonus))))
	totalScore := K * quantity

	return totalScore, sat
}

// ApplyChefData computes a chef's derived skill values, limit effects, and material reductions.
func ApplyChefData(
	chef *Chef,
	equip *Equip,
	useEquip bool,
	globalUltimate []SkillEffect,
	partialAdds []SkillEffect,
	selfUltimateData []SelfUltimateEntry,
	activityUltimate []SkillEffect,
	useDisk bool,
	rule *Rule,
	qixia map[int]*QixiaEntry,
) {
	if chef == nil || chef.ChefID == 0 {
		return
	}

	var stirAdd, boilAdd, knifeAdd, fryAdd, bakeAdd, steamAdd Addition
	var maxEquipLimits []LimitEffect
	var matReduces []MaterialReduce

	chef.SelfUltimateEffect = nil
	var C []SkillEffect

	if !rule.DisableChefSkillEffect {
		C = append(C, chef.SpecialSkillEffect...)
		C = append(C, globalUltimate...)

		for _, su := range selfUltimateData {
			if su.ChefID == chef.ChefID {
				chef.SelfUltimateEffect = su.Effect
				C = append(C, su.Effect...)
				break
			}
		}

		for i := range partialAdds {
			pass, _ := checkSkillCondition(&partialAdds[i], chef, nil, nil, 0, nil)
			if pass {
				C = append(C, partialAdds[i])
			}
		}
	}

	if !rule.DisableEquipSkillEffect && useEquip && equip != nil && len(equip.Effect) > 0 {
		eqEffects := updateEquipmentEffect(equip.Effect, chef.SelfUltimateEffect)
		C = append(C, eqEffects...)
	}

	C = append(C, activityUltimate...)

	if useDisk {
		for _, amber := range chef.Disk.Ambers {
			if amber.Data != nil {
				level := chef.Disk.Level
				if level > 0 && level <= len(amber.Data.AllEffect) {
					C = append(C, amber.Data.AllEffect[level-1]...)
				}
			}
		}
	}

	for i := range C {
		eff := &C[i]
		if eff.Tag != 0 && !intSliceContains(chef.Tags, eff.Tag) {
			continue
		}
		switch eff.Type {
		case "Stirfry":
			setAddition(&stirAdd, eff)
		case "Boil":
			setAddition(&boilAdd, eff)
		case "Knife":
			setAddition(&knifeAdd, eff)
		case "Fry":
			setAddition(&fryAdd, eff)
		case "Bake":
			setAddition(&bakeAdd, eff)
		case "Steam":
			setAddition(&steamAdd, eff)
		case "MaxEquipLimit":
			if eff.Condition == "Self" || eff.Condition == "Partial" {
				maxEquipLimits = append(maxEquipLimits, LimitEffect{
					Rarity: eff.Rarity,
					Value:  int(eff.Value),
				})
			}
		case "MaterialReduce":
			if eff.Condition == "Self" || eff.Condition == "Partial" {
				matReduces = append(matReduces, MaterialReduce{
					ConditionType:      eff.Type,
					ConditionValueList: conditionValueListInts(eff.ConditionValueList),
					Cal:                eff.Cal,
					Value:              eff.Value,
				})
			}
		}
	}

	if qixia != nil {
		for _, tag := range chef.Tags {
			if q, ok := qixia[tag]; ok {
				stirAdd.Abs += q.Stirfry
				boilAdd.Abs += q.Boil
				knifeAdd.Abs += q.Knife
				fryAdd.Abs += q.Fry
				bakeAdd.Abs += q.Bake
				steamAdd.Abs += q.Steam
			}
		}
	}

	chef.StirfryVal = math.Ceil(calAddition(float64(chef.Stirfry), stirAdd))
	chef.BoilVal = math.Ceil(calAddition(float64(chef.Boil), boilAdd))
	chef.KnifeVal = math.Ceil(calAddition(float64(chef.Knife), knifeAdd))
	chef.FryVal = math.Ceil(calAddition(float64(chef.Fry), fryAdd))
	chef.BakeVal = math.Ceil(calAddition(float64(chef.Bake), bakeAdd))
	chef.SteamVal = math.Ceil(calAddition(float64(chef.Steam), steamAdd))

	chef.MaxLimitEffect = maxEquipLimits
	chef.MaterialEffects = matReduces
}

// ── resolveState: convert index-based SimState to object-based arrays for scoring ──
// Returns cloned chef objects (since ApplyChefData mutates them), recipe slots, and equip pointers.

type resolvedSlot struct {
	chefObj  *Chef
	equipObj *Equip
	recipes  [3]recipeSlot
}

func resolveRuleState(rule *Rule, rs RuleState) []resolvedSlot {
	slots := make([]resolvedSlot, len(rs))
	for ci := range rs {
		ss := &rs[ci]
		var chef *Chef
		var equip *Equip
		if ss.ChefIdx >= 0 && ss.ChefIdx < len(rule.Chefs) {
			// Clone chef so ApplyChefData mutation is local
			orig := rule.Chefs[ss.ChefIdx]
			clone := orig
			// Deep-copy slices that ApplyChefData writes to
			clone.SelfUltimateEffect = nil
			clone.MaxLimitEffect = nil
			clone.MaterialEffects = nil
			chef = &clone
			slots[ci].chefObj = chef

			// Resolve equip: use embedded equipEffect from preprocessor
			equip = orig.EquipEffect
		}
		slots[ci].equipObj = equip

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
	return slots
}

// ── applyChefDataForRule: JS _applyChefData (line 844) ──

func applyChefDataForRule(rule *Rule, slots []resolvedSlot) {
	customArr := buildCustomArr(slots)
	partialAdds := getPartialChefAdds(customArr, rule)

	for ci := range slots {
		chef := slots[ci].chefObj
		if chef != nil && chef.ChefID != 0 {
			var pa []SkillEffect
			if ci < len(partialAdds) {
				pa = partialAdds[ci]
			}
			ApplyChefData(
				chef,
				slots[ci].equipObj,
				true,
				rule.CalGlobalUltimateData,
				pa,
				rule.CalSelfUltimateData,
				rule.CalActivityUltimateData,
				true,
				rule,
				rule.CalQixiaData,
			)
		}
	}
}

// CalcRuleScore computes the total score for a single guest rule given the current assignments.
func CalcRuleScore(rules []Rule, simState SimState, ruleIndex int, gameData *GameData) int {
	if ruleIndex < 0 || ruleIndex >= len(rules) || ruleIndex >= len(simState) {
		return 0
	}
	rule := &rules[ruleIndex]
	rs := simState[ruleIndex]

	// Resolve indices to objects
	slots := resolveRuleState(rule, rs)

	// Apply chef data (compute skill values)
	applyChefDataForRule(rule, slots)

	// Recompute recipe quantities (JS lines 891-901)
	// Clone material pool so each slot's consumption is tracked,
	// preventing shared materials from being double-counted.
	matPool := CloneMaterials(rule.Materials)
	for ci := range slots {
		for reci := 0; reci < 3; reci++ {
			rec := &slots[ci].recipes[reci]
			if rec.Data != nil {
				recipeMax := GetRecipeQuantity(rec.Data, matPool, rule, slots[ci].chefObj)
				if rule.DisableMultiCookbook && recipeMax > 1 {
					recipeMax = 1
				}
				rec.Max = recipeMax
				if rec.Quantity > recipeMax {
					rec.Quantity = recipeMax
				}
				UpdateMaterialsData(matPool, rec.Data, rec.Quantity, slots[ci].chefObj)
			}
		}
	}

	customArr := buildCustomArr(slots)
	partialRecipeAdds := getPartialRecipeAdds(customArr, rule)
	intentAdds := getIntentAdds(ruleIndex, customArr, gameData, rules)

	// Sum scores (JS lines 917-942)
	u := 0
	for ci := range slots {
		for reci := 0; reci < 3; reci++ {
			rec := &slots[ci].recipes[reci]
			if rec.Data != nil {
				var pa []PartialAdd
				idx := 3*ci + reci
				if idx < len(partialRecipeAdds) {
					pa = partialRecipeAdds[idx]
				}
				var ia []Intent
				if idx < len(intentAdds) {
					ia = intentAdds[idx]
				}

				totalScore, sat := getRecipeScore(
					slots[ci].chefObj,
					slots[ci].equipObj,
					rec.Data,
					rec.Quantity,
					rec.Max,
					rule,
					&slots[ci].recipes,
					pa,
					ia,
				)

				actAdd := rec.Data.ActivityAddition
				u += int(math.Ceil(toFixed2(float64(totalScore) * (1 + actAdd/100))))
				rec.Satiety = sat
			}
		}
	}

	// Score modifiers (JS lines 944-953)
	h := 1.0
	if rule.ScoreMultiply != 0 {
		h = rule.ScoreMultiply
	}
	m := 1.0
	if rule.ScorePow != 0 {
		m = rule.ScorePow
	}
	v := 0
	if rule.ScoreAdd != 0 {
		v = rule.ScoreAdd
	}

	uf := toFixed2(math.Pow(float64(u), m) * h)
	if rule.IsActivity {
		u = int(math.Ceil(uf))
	} else {
		u = int(math.Floor(uf))
	}
	if u != 0 {
		u += v
	}

	// Satiety bonus (JS lines 957-973)
	if rule.Satiety != 0 {
		satTotal := 0
		satCount := 0
		expected := 3 * len(rule.IntentList)
		for ci := range slots {
			for reci := 0; reci < 3; reci++ {
				if slots[ci].recipes[reci].Data != nil {
					satTotal += slots[ci].recipes[reci].Satiety
					satCount++
				}
			}
		}
		if satCount == expected {
			satAdd := getSatietyPercent(satTotal, rule)
			u = calSatietyAdd(u, satAdd)
		}
	}

	return u
}

// FastCalcScore sums CalcRuleScore across all rules for a complete contest score.
func FastCalcScore(rules []Rule, simState SimState, gameData *GameData) int {
	total := 0
	for ri := range rules {
		total += CalcRuleScore(rules, simState, ri, gameData)
	}
	return total
}

// ── getSatietyPercent: JS line 7202 ──

func getSatietyPercent(satTotal int, rule *Rule) float64 {
	if rule == nil {
		return 0
	}
	if rule.SatisfyRewardType == 1 && satTotal == rule.Satiety {
		return rule.SatisfyExtraValue
	}
	return -rule.SatisfyDeductValue * math.Abs(float64(satTotal-rule.Satiety))
}

// ── calSatietyAdd: JS line 7222 ──

func calSatietyAdd(score int, satAdd float64) int {
	return int(math.Ceil(toFixed2(float64(score) * (1 + 0.01*satAdd))))
}

// ── Helpers ──

func buildCustomArr(slots []resolvedSlot) []customEntry {
	arr := make([]customEntry, len(slots))
	for ci := range slots {
		arr[ci] = customEntry{
			Chef:    slots[ci].chefObj,
			Equip:   slots[ci].equipObj,
			Recipes: &slots[ci].recipes,
		}
	}
	return arr
}

func intSliceContains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func conditionValueInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		if n, err := strconv.Atoi(x); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return int(f)
		}
	}
	return 0
}

func interfaceToInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		if n, err := strconv.Atoi(x); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return int(f)
		}
	}
	return 0
}

func interfaceToString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func conditionValueListInts(list []interface{}) []int {
	result := make([]int, len(list))
	for i, v := range list {
		result[i] = interfaceToInt(v)
	}
	return result
}

// ── Material helpers for the search layer ──

// CloneMaterials returns a shallow copy of a material slice.
func CloneMaterials(mats []Material) []Material {
	c := make([]Material, len(mats))
	copy(c, mats)
	return c
}

// UpdateMaterialsData subtracts the materials consumed by cooking a recipe the given number of times.
func UpdateMaterialsData(materials []Material, recipe *Recipe, quantity int, chef *Chef) {
	if recipe == nil {
		return
	}
	for _, rm := range recipe.Materials {
		for j := range materials {
			if rm.Material == materials[j].MaterialID && materials[j].Quantity > 0 {
				reduced := calMaterialReduce(chef, rm.Material, rm.Quantity)
				materials[j].Quantity -= reduced * quantity
			}
		}
	}
}
