package main

import (
	"fmt"
	"math"
	"strings"
)

// RecipeDetail holds per-recipe scoring breakdown for formatted output.
type RecipeDetail struct {
	Name       string
	TotalScore int // score * quantity, after activityAddition
	Quantity   int
	Satiety    int
}

// ChefDetail holds per-chef scoring breakdown for formatted output.
type ChefDetail struct {
	Name       string
	TotalScore int // sum of 3 recipe TotalScores
	TotalQty   int // sum of 3 recipe quantities
	Recipes    [3]RecipeDetail
}

// RuleDetail holds per-guest scoring breakdown for formatted output.
type RuleDetail struct {
	Title      string
	GuestScore int // final score after scoreMultiply/scorePow/scoreAdd/satiety
	Satiety    int // actual satiety (sum of recipe satieties)
	ReqSatiety int // required satiety from rule
	Chefs      []ChefDetail
}

// CalcRuleDetail computes the same score as CalcRuleScore but also returns
// per-chef/recipe breakdown for formatted output.
func CalcRuleDetail(rules []Rule, simState SimState, ruleIndex int, gameData *GameData) RuleDetail {
	rule := &rules[ruleIndex]
	rs := simState[ruleIndex]

	detail := RuleDetail{
		Title:      rule.Title,
		ReqSatiety: rule.Satiety,
	}

	slots := resolveRuleState(rule, rs)
	applyChefDataForRule(rule, slots)

	// Recompute recipe quantities with material tracking (must match CalcRuleScore).
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

	detail.Chefs = make([]ChefDetail, len(slots))
	u := 0
	satTotal := 0

	for ci := range slots {
		cd := &detail.Chefs[ci]
		if slots[ci].chefObj != nil {
			cd.Name = slots[ci].chefObj.Name
		}

		for reci := 0; reci < 3; reci++ {
			rec := &slots[ci].recipes[reci]
			rd := &cd.Recipes[reci]

			if rec.Data == nil {
				continue
			}

			rd.Name = rec.Data.Name
			rd.Quantity = rec.Quantity

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
			adjusted := int(math.Ceil(toFixed2(float64(totalScore) * (1 + actAdd/100))))

			rd.TotalScore = adjusted
			rd.Satiety = sat
			cd.TotalScore += adjusted
			cd.TotalQty += rec.Quantity
			u += adjusted
			satTotal += sat
		}
	}

	// Score modifiers
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

	// Satiety bonus
	if rule.Satiety != 0 {
		satCount := 0
		expected := 3 * len(rule.IntentList)
		for ci := range slots {
			for reci := 0; reci < 3; reci++ {
				if slots[ci].recipes[reci].Data != nil {
					satCount++
				}
			}
		}
		if satCount == expected {
			satAdd := getSatietyPercent(satTotal, rule)
			u = calSatietyAdd(u, satAdd)
		}
	}

	detail.GuestScore = u
	detail.Satiety = satTotal
	return detail
}

// FormatResult produces calculator-compatible text output for a contest result.
func FormatResult(rules []Rule, simState SimState, gameData *GameData, contest *Contest) string {
	var b strings.Builder

	for ri := range rules {
		if ri > 0 {
			b.WriteString("===================\n")
		}

		d := CalcRuleDetail(rules, simState, ri, gameData)

		fmt.Fprintf(&b, "第%d位客人：%s %d / %d -> %d\n",
			ri+1, d.Title, d.Satiety, d.ReqSatiety, d.GuestScore)

		for ci := range d.Chefs {
			cd := &d.Chefs[ci]
			if cd.Name == "" {
				continue
			}

			fmt.Fprintf(&b, "厨师：%s-设定厨具 -> %d / %d\n",
				cd.Name, cd.TotalQty, cd.TotalScore)

			var recipeParts []string
			for reci := 0; reci < 3; reci++ {
				rd := &cd.Recipes[reci]
				if rd.Name == "" {
					continue
				}
				recipeParts = append(recipeParts, fmt.Sprintf("%s(%d)", rd.Name, rd.TotalScore))
			}
			if len(recipeParts) > 0 {
				fmt.Fprintf(&b, "菜谱：%s\n", strings.Join(recipeParts, "；"))
			}
		}
	}

	return b.String()
}
