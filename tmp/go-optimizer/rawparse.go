package main

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
)

type skillEntry struct {
	effects []SkillEffect
}

type diskInfo struct {
	maxLevel int
	info     []int // slot types
}

type amberInfo struct {
	typ           int
	skill         []int
	amplification int
}

type equipInfo struct {
	equipID int
	effects []SkillEffect
}

type iRecipe struct {
	Recipe
	BasePrice float64
	ExPrice   float64
	Limit     int
}

type iChef struct {
	ChefID              int
	Name                string
	Rarity              int
	Addition            float64
	Tags                []int
	EquipID             int
	Stirfry, Boil, Knife, Fry, Bake, Steam int
	Got                 bool
	SpecialSkillEffect  []SkillEffect
	UltimateSkillEffect []SkillEffect
	UltimateSkillList   []int
	BaseChefID          int
	RawDiskID           int
	Disk                Disk
}

type iMaterial struct {
	MaterialID int
	Name       string
	Origin     OriginType
}

type ultimateResult struct {
	global  []SkillEffect
	partial []SelfUltimateEntry
	self    []SelfUltimateEntry
	qixia   map[int]*QixiaEntry
}

type gameCache struct {
	recipes   []iRecipe
	chefs     []iChef
	equips    []equipInfo
	equipByID []*equipInfo // indexed by equipID
	materials []iMaterial

	skillMap map[int]*skillEntry
	diskMap  map[int]*diskInfo
	amberMap map[int]*amberInfo

	decorationEffect float64
	ultimateData     *ultimateResult
}

func buildSkillTable(dataJSON string) map[int]*skillEntry {
	m := make(map[int]*skillEntry)
	gjson.Get(dataJSON, "skills").ForEach(func(_, v gjson.Result) bool {
		id := int(v.Get("skillId").Int())
		var effects []SkillEffect
		v.Get("effect").ForEach(func(_, e gjson.Result) bool {
			effects = append(effects, parseSkillEffect(e))
			return true
		})
		m[id] = &skillEntry{effects: effects}
		return true
	})
	return m
}

func parseSkillEffect(e gjson.Result) SkillEffect {
	se := SkillEffect{
		Type:          parseEffType(e.Get("type").String()),
		Cal:           parseCalType(e.Get("cal").String()),
		Value:         e.Get("value").Float(),
		Rarity:        int(e.Get("rarity").Int()),
		Condition:     parseCondScope(e.Get("condition").String()),
		ConditionType: parseCondType(e.Get("conditionType").String()),
		Tag:           int(e.Get("tag").Int()),
	}
	cv := e.Get("conditionValue")
	if cv.Exists() {
		se.ConditionValueInt = int(cv.Int())
	}
	e.Get("conditionValueList").ForEach(func(_, item gjson.Result) bool {
		se.ConditionValueList = append(se.ConditionValueList, int(item.Int()))
		return true
	})
	return se
}

func resolveEffects(sm map[int]*skillEntry, ids []int) []SkillEffect {
	var out []SkillEffect
	for _, id := range ids {
		if s, ok := sm[id]; ok {
			out = append(out, s.effects...)
		}
	}
	return out
}

func loadFromStrings(dataJSON, archJSON string) *InputData {
	gc := buildGameCache(dataJSON)
	maxEquipID := 0
	for i := range gc.equips {
		if gc.equips[i].equipID > maxEquipID {
			maxEquipID = gc.equips[i].equipID
		}
	}
	gc.equipByID = make([]*equipInfo, maxEquipID+1)
	for i := range gc.equips {
		gc.equipByID[gc.equips[i].equipID] = &gc.equips[i]
	}
	applyArchive(gc, archJSON)
	gc.ultimateData = computeUltimateData(gc, archJSON)
	return buildOutput(gc, dataJSON, archJSON)
}

func LoadRawData(dataPath, archivePath string) (*InputData, error) {
	rawBytes, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dataPath, err)
	}
	archBytes, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", archivePath, err)
	}
	return loadFromStrings(string(rawBytes), string(archBytes)), nil
}

func buildGameCache(dataJSON string) *gameCache {
	sm := buildSkillTable(dataJSON)

	diskMap := make(map[int]*diskInfo)
	gjson.Get(dataJSON, "disks").ForEach(func(_, v gjson.Result) bool {
		id := int(v.Get("diskId").Int())
		var info []int
		v.Get("info").ForEach(func(_, slot gjson.Result) bool {
			info = append(info, int(slot.Int()))
			return true
		})
		diskMap[id] = &diskInfo{maxLevel: int(v.Get("maxLevel").Int()), info: info}
		return true
	})

	amberMap := make(map[int]*amberInfo)
	gjson.Get(dataJSON, "ambers").ForEach(func(_, v gjson.Result) bool {
		id := int(v.Get("amberId").Int())
		var skillIDs []int
		v.Get("skill").ForEach(func(_, s gjson.Result) bool {
			skillIDs = append(skillIDs, int(s.Int()))
			return true
		})
		amberMap[id] = &amberInfo{
			typ:           int(v.Get("type").Int()),
			skill:         skillIDs,
			amplification: int(v.Get("amplification").Int()),
		}
		return true
	})

	type matSort struct {
		id     int
		name   string
		origin string
	}
	var mats []matSort
	gjson.Get(dataJSON, "materials").ForEach(func(_, v gjson.Result) bool {
		mats = append(mats, matSort{
			id:     int(v.Get("materialId").Int()),
			name:   v.Get("name").String(),
			origin: v.Get("origin").String(),
		})
		return true
	})
	slices.SortStableFunc(mats, func(a, b matSort) int { return cmp.Compare(a.origin, b.origin) })
	materials := make([]iMaterial, len(mats))
	matOriginMap := make(map[int]OriginType, len(mats))
	for i, m := range mats {
		o := parseOriginType(m.origin)
		materials[i] = iMaterial{MaterialID: m.id, Name: m.name, Origin: o}
		matOriginMap[m.id] = o
	}

	var equips []equipInfo
	gjson.Get(dataJSON, "equips").ForEach(func(_, v gjson.Result) bool {
		if v.Get("name").String() == "" {
			return true
		}
		var skillIDs []int
		v.Get("skill").ForEach(func(_, s gjson.Result) bool {
			skillIDs = append(skillIDs, int(s.Int()))
			return true
		})
		equips = append(equips, equipInfo{
			equipID: int(v.Get("equipId").Int()),
			effects: resolveEffects(sm, skillIDs),
		})
		return true
	})

	var chefs []iChef
	gjson.Get(dataJSON, "chefs").ForEach(func(_, v gjson.Result) bool {
		if v.Get("name").String() == "" {
			return true
		}
		tags := readIntSlice(v.Get("tags"))
		if tags == nil {
			tags = []int{}
		}

		skillID := int(v.Get("skill").Int())
		var specialEffect []SkillEffect
		if skillID != 0 {
			specialEffect = resolveEffects(sm, []int{skillID})
		}

		ultList := readIntSlice(v.Get("ultimateSkillList"))
		ultExtra := readIntSlice(v.Get("ultimateSkillExtra"))
		allUlt := append(append([]int{}, ultList...), ultExtra...)
		ultimateEffect := resolveEffects(sm, allUlt)

		diskID := int(v.Get("disk").Int())
		disk := Disk{Level: 1, MaxLevel: 1}
		if d, ok := diskMap[diskID]; ok {
			disk.MaxLevel = d.maxLevel
			disk.Ambers = make([]Amber, len(d.info))
		}

		chefs = append(chefs, iChef{
			ChefID:              int(v.Get("chefId").Int()),
			Name:                v.Get("name").String(),
			Rarity:              int(v.Get("rarity").Int()),
			Tags:                tags,
			Stirfry:             int(v.Get("stirfry").Int()),
			Boil:                int(v.Get("boil").Int()),
			Knife:               int(v.Get("knife").Int()),
			Fry:                 int(v.Get("fry").Int()),
			Bake:                int(v.Get("bake").Int()),
			Steam:               int(v.Get("steam").Int()),
			SpecialSkillEffect:  specialEffect,
			UltimateSkillEffect: ultimateEffect,
			UltimateSkillList:   ultList,
			BaseChefID:          int(v.Get("baseChefId").Int()),
			RawDiskID:           diskID,
			Disk:                disk,
		})
		return true
	})

	var recipes []iRecipe
	gjson.Get(dataJSON, "recipes").ForEach(func(_, v gjson.Result) bool {
		if v.Get("name").String() == "" {
			return true
		}
		tags := readIntSlice(v.Get("tags"))
		if tags == nil {
			tags = []int{}
		}
		var mats []RecipeMaterial
		v.Get("materials").ForEach(func(_, m gjson.Result) bool {
			mid := int(m.Get("material").Int())
			mats = append(mats, RecipeMaterial{
				Material: mid,
				Quantity: int(m.Get("quantity").Int()),
				Origin:   matOriginMap[mid],
			})
			return true
		})
		price := v.Get("price").Float()
		limit := int(v.Get("limit").Int())
		recipes = append(recipes, iRecipe{
			Recipe: Recipe{
				RecipeID:  int(v.Get("recipeId").Int()),
				Name:      v.Get("name").String(),
				Rarity:    int(v.Get("rarity").Int()),
				Price:     price,
				Stirfry:   int(v.Get("stirfry").Int()),
				Boil:      int(v.Get("boil").Int()),
				Knife:     int(v.Get("knife").Int()),
				Fry:       int(v.Get("fry").Int()),
				Bake:      int(v.Get("bake").Int()),
				Steam:     int(v.Get("steam").Int()),
				Materials: mats,
				Condiment: parseCondimentType(v.Get("condiment").String()),
				Tags:      tags,
				TagSet:    intsToBoolSet(tags),
				LimitVal:  limit,
			},
			BasePrice: price,
			ExPrice:   v.Get("exPrice").Float(),
			Limit:     limit,
		})
		computeRecipeMasks(&recipes[len(recipes)-1].Recipe)
		return true
	})

	return &gameCache{
		recipes:   recipes,
		chefs:     chefs,
		equips:    equips,
		materials: materials,
		skillMap:  sm,
		diskMap:   diskMap,
		amberMap:  amberMap,
	}
}

func readIntSlice(v gjson.Result) []int {
	if !v.Exists() || !v.IsArray() {
		return nil
	}
	arr := v.Array()
	out := make([]int, len(arr))
	for i, item := range arr {
		out[i] = int(item.Int())
	}
	return out
}

func applyArchive(gc *gameCache, archJSON string) {
	msg := gjson.Get(archJSON, "msg")

	type archRec struct{ got, ex string }
	archRecipes := make(map[int]archRec)
	msg.Get("recipes").ForEach(func(_, v gjson.Result) bool {
		archRecipes[int(v.Get("id").Int())] = archRec{
			got: v.Get("got").String(),
			ex:  v.Get("ex").String(),
		}
		return true
	})
	for i := range gc.recipes {
		r := &gc.recipes[i]
		if ar, ok := archRecipes[r.RecipeID]; ok {
			if ar.got == "是" || ar.got == "true" {
				r.Got = true
			}
			if ar.ex == "是" {
				r.Price = r.BasePrice + r.ExPrice
			}
		}
	}

	equipMap := gc.equipByID

	type archCh struct {
		got, ult string
		equip    int
		dlv      int
		ambers   []int
	}
	archChefs := make(map[int]archCh)
	msg.Get("chefs").ForEach(func(_, v gjson.Result) bool {
		archChefs[int(v.Get("id").Int())] = archCh{
			got:   v.Get("got").String(),
			ult:   v.Get("ult").String(),
			equip: int(v.Get("equip").Int()),
			dlv:   int(v.Get("dlv").Int()),
			ambers: readIntSlice(v.Get("ambers")),
		}
		return true
	})

	for i := range gc.chefs {
		c := &gc.chefs[i]
		ac, ok := archChefs[c.ChefID]
		if !ok {
			continue
		}
		if ac.got == "是" || ac.got == "true" {
			c.Got = true
		}
		if ac.equip != 0 && ac.equip < len(equipMap) {
			if eq := equipMap[ac.equip]; eq != nil {
				c.EquipID = eq.equipID
			}
		}
		if ac.dlv > 0 && ac.dlv <= c.Disk.MaxLevel {
			c.Disk.Level = ac.dlv
		} else {
			c.Disk.Level = 1
		}

		// Amber slot matching
		slotTypes := gc.diskMap[c.RawDiskID]
		if len(ac.ambers) > 0 && slotTypes != nil && len(slotTypes.info) > 0 {
			remaining := make([]int, len(ac.ambers))
			copy(remaining, ac.ambers)
			for si := range c.Disk.Ambers {
				matched := false
				for ri := 0; ri < len(remaining); ri++ {
					if remaining[ri] == 0 {
						remaining = append(remaining[:ri], remaining[ri+1:]...)
						break
					}
					if amb, ok := gc.amberMap[remaining[ri]]; ok {
						if si < len(slotTypes.info) && amb.typ == slotTypes.info[si] {
							c.Disk.Ambers[si].Data = &AmberData{AllEffect: amberAllEffects(gc.skillMap, amb)}
							remaining = append(remaining[:ri], remaining[ri+1:]...)
							matched = true
							break
						}
					}
				}
				if !matched {
					c.Disk.Ambers[si].Data = nil
				}
			}
		} else {
			for si := range c.Disk.Ambers {
				c.Disk.Ambers[si].Data = nil
			}
		}
	}

	if de := msg.Get("decorationEffect"); de.Exists() {
		gc.decorationEffect = de.Float()
	}
}

func amberAllEffects(sm map[int]*skillEntry, amb *amberInfo) [][]SkillEffect {
	base := resolveEffects(sm, amb.skill)
	allEffect := make([][]SkillEffect, 5)
	for lvl := 1; lvl <= 5; lvl++ {
		levelEffects := make([]SkillEffect, len(base))
		for i, e := range base {
			levelEffects[i] = e
			levelEffects[i].Value = e.Value + float64((lvl-1)*amb.amplification)
		}
		allEffect[lvl-1] = levelEffects
	}
	return allEffect
}

func computeUltimateData(gc *gameCache, archJSON string) *ultimateResult {
	archChefUlt := make(map[int]string) // chefID -> ult value
	archChefGot := make(map[int]string) // chefID -> got value
	gjson.Get(archJSON, "msg.chefs").ForEach(func(_, v gjson.Result) bool {
		id := int(v.Get("id").Int())
		archChefUlt[id] = v.Get("ult").String()
		archChefGot[id] = v.Get("got").String()
		return true
	})

	var globalEffects []SkillEffect
	var partialEntries []SelfUltimateEntry
	var selfEntries []SelfUltimateEntry
	qixia := make(map[int]*QixiaEntry)

	for i := range gc.chefs {
		c := &gc.chefs[i]
		if len(c.UltimateSkillEffect) == 0 && len(c.UltimateSkillList) == 0 {
			continue
		}
		if archChefUlt[c.ChefID] != "是" {
			continue
		}

		var partialEffects, selfEffects []SkillEffect
		for _, eff := range c.UltimateSkillEffect {
			switch eff.Condition {
			case CondScopePartial, CondScopeNext:
				partialEffects = append(partialEffects, eff)
			case CondScopeGlobal:
				merged := false
				for gi := range globalEffects {
					g := &globalEffects[gi]
					if g.Type == eff.Type && g.Cal == eff.Cal && g.Tag == eff.Tag && g.Rarity == eff.Rarity {
						g.Value = toFixed2(g.Value + eff.Value)
						merged = true
						break
					}
				}
				if !merged {
					globalEffects = append(globalEffects, eff)
				}
			case CondScopeSelf:
				selfEffects = append(selfEffects, eff)
			}
		}

		if len(partialEffects) > 0 {
			partialEntries = append(partialEntries, SelfUltimateEntry{ChefID: c.ChefID, Effect: partialEffects})
		}
		if len(selfEffects) > 0 {
			selfEntries = append(selfEntries, SelfUltimateEntry{ChefID: c.ChefID, Effect: selfEffects})
		}

		// SwordsUnited (qixia)
		for _, skillID := range c.UltimateSkillList {
			se, ok := gc.skillMap[skillID]
			if !ok || len(se.effects) == 0 || se.effects[0].ConditionType != CondTypeSwordsUnited {
				continue
			}
			eff0 := se.effects[0]
			if len(eff0.ConditionValueList) == 0 {
				continue
			}

			matchCount := 0
			for _, condVal := range eff0.ConditionValueList {
				for j := range gc.chefs {
					if gc.chefs[j].BaseChefID == condVal {
						oc := gc.chefs[j].ChefID
						if archChefGot[oc] != "" && archChefUlt[oc] == "是" {
							matchCount++
							break
						}
					}
				}
			}

			if matchCount >= len(eff0.ConditionValueList) && eff0.Tag != 0 {
				if _, exists := qixia[eff0.Tag]; !exists {
					qixia[eff0.Tag] = &QixiaEntry{}
				}
				q := qixia[eff0.Tag]
				for _, se := range se.effects {
					if se.Tag == eff0.Tag {
						switch se.Type {
						case EffStirfry:
							q.Stirfry += se.Value
						case EffBoil:
							q.Boil += se.Value
						case EffKnife:
							q.Knife += se.Value
						case EffFry:
							q.Fry += se.Value
						case EffBake:
							q.Bake += se.Value
						case EffSteam:
							q.Steam += se.Value
						}
					}
				}
			}
		}
	}

	return &ultimateResult{global: globalEffects, partial: partialEntries, self: selfEntries, qixia: qixia}
}

func buildOutput(gc *gameCache, dataJSON, archJSON string) *InputData {
	calGlobal := buildCalGlobalUltimate(gc.ultimateData.global)

	partialChefIDs := make([]int, 0, len(gc.ultimateData.partial))
	for _, p := range gc.ultimateData.partial {
		partialChefIDs = append(partialChefIDs, p.ChefID)
	}
	selfChefIDs := make([]int, 0, len(gc.ultimateData.self))
	for _, s := range gc.ultimateData.self {
		selfChefIDs = append(selfChefIDs, s.ChefID)
	}
	calSelf := buildCalSelfUltimate(gc, selfChefIDs)

	exRecipeSet := make(map[int]bool)
	for i := range gc.recipes {
		if gc.recipes[i].Price > gc.recipes[i].BasePrice {
			exRecipeSet[gc.recipes[i].RecipeID] = true
		}
	}

	type contestInfo struct {
		name     string
		ruleID   int
		groupIDs []int
	}
	var contests []contestInfo
	gjson.Get(dataJSON, "rules").ForEach(func(_, v gjson.Result) bool {
		id := int(v.Get("Id").Int())
		if id < 680000 || id >= 690000 {
			return true
		}
		title := strings.TrimPrefix(v.Get("Title").String(), "风云宴 ")
		cleaned := strings.ReplaceAll(title, " ", "+")
		contests = append(contests, contestInfo{name: cleaned, ruleID: id, groupIDs: readIntSliceFromArray(v.Get("Group"))})
		return true
	})
	slices.SortFunc(contests, func(a, b contestInfo) int { return a.ruleID - b.ruleID })
	fmt.Fprintf(os.Stderr, "Found %d contests\n", len(contests))

	ruleIndex := make(map[int]gjson.Result)
	gjson.Get(dataJSON, "rules").ForEach(func(_, v gjson.Result) bool {
		ruleIndex[int(v.Get("Id").Int())] = v
		return true
	})

	output := &InputData{}
	for ci, cinfo := range contests {
		fmt.Fprintf(os.Stderr, "Processing contest %d/%d: %s\n", ci+1, len(contests), cinfo.name)
		contest := Contest{RuleID: cinfo.ruleID, Name: cinfo.name}
		for _, gid := range cinfo.groupIDs {
			if gr, ok := ruleIndex[gid]; ok {
				rule := processGuestRule(gc, gr, calGlobal, partialChefIDs, calSelf, exRecipeSet)
				contest.Rules = append(contest.Rules, rule)
			}
		}
		output.Contests = append(output.Contests, contest)
	}

	output.Global = globalData{
		Intents: parseIntentArray(dataJSON, "intents"),
		Buffs:   parseIntentArray(dataJSON, "buffs"),
	}
	return output
}

func processGuestRule(
	gc *gameCache,
	gr gjson.Result,
	calGlobal []SkillEffect,
	partialChefIDs []int,
	calSelf []SelfUltimateEntry,
	exRecipeSet map[int]bool,
) Rule {
	rule := Rule{
		Title:                   gr.Get("Title").String(),
		Satiety:                 int(gr.Get("Satiety").Int()),
		DisableMultiCookbook:    toBool(gr.Get("DisableMultiCookbook")),
		DisableCookbookRank:     toBool(gr.Get("DisableCookbookRank")),
		DisableChefSkillEffect:  toBool(gr.Get("DisableChefSkillEffect")),
		DisableEquipSkillEffect: toBool(gr.Get("DisableEquipSkillEffect")),
		DisableDecorationEffect: toBool(gr.Get("DisableDecorationEffect")),
		IsActivity:              toBool(gr.Get("IsActivity")),
		SatisfyRewardType:       int(gr.Get("SatisfyRewardType").Int()),
		SatisfyExtraValue:       gr.Get("SatisfyExtraValue").Float(),
		SatisfyDeductValue:      gr.Get("SatisfyDeductValue").Float(),
		DecorationEffect:        gc.decorationEffect,
		CalGlobalUltimateData:   calGlobal,
		CalPartialChefSet:       intsToBoolSet(partialChefIDs),
		CalSelfUltimateData:     calSelf,
		CalQixiaData:            gc.ultimateData.qixia,
	}

	gr.Get("IntentList").ForEach(func(_, row gjson.Result) bool {
		rule.IntentList = append(rule.IntentList, readIntSliceFromArray(row))
		return true
	})

	rule.RecipeEffect = readIntFloatMap(gr.Get("RecipeEffect"))
	rule.ChefTagEffect = readIntFloatMap(gr.Get("ChefTagEffect"))

	rule.GlobalBuffList = readIntSliceFromArray(gr.Get("GlobalBuffList"))

	mats := make([]Material, len(gc.materials))
	for i, m := range gc.materials {
		mats[i] = Material{MaterialID: m.MaterialID, Name: m.Name, Origin: m.Origin}
	}
	hasMaterialsEffect := false
	gr.Get("MaterialsEffect").ForEach(func(_, v gjson.Result) bool {
		mid := int(v.Get("MaterialID").Int())
		eff := v.Get("Effect").Float()
		for mi := range mats {
			if mats[mi].MaterialID == mid {
				mats[mi].Addition = toFixed2(mats[mi].Addition + eff)
				hasMaterialsEffect = true
				break
			}
		}
		return true
	})
	if matNums := gr.Get("MaterialsNum"); matNums.Exists() && matNums.IsArray() && len(matNums.Array()) > 0 {
		var filtered []Material
		matNums.ForEach(func(_, v gjson.Result) bool {
			mid := int(v.Get("MaterialID").Int())
			num := int(v.Get("Num").Int())
			for mi := range mats {
				if mats[mi].MaterialID == mid {
					if num != 1 {
						mats[mi].Quantity = num
					}
					filtered = append(filtered, mats[mi])
					break
				}
			}
			return true
		})
		mats = filtered
	}
	rule.Materials = mats
	rule.MaterialsEffect = hasMaterialsEffect

	cookbookRarityLimit := int(gr.Get("CookbookRarityLimit").Int())
	hasCookbookLimit := gr.Get("CookbookRarityLimit").Exists()
	chefRarityLimit := int(gr.Get("ChefRarityLimit").Int())
	hasChefLimit := gr.Get("ChefRarityLimit").Exists()

	type tagEffect struct{ tagID int; effect float64 }
	type skillEffect struct{ skill CookSkill; effect float64 }

	readTagEffects := func(key string) []tagEffect {
		var out []tagEffect
		gr.Get(key).ForEach(func(_, v gjson.Result) bool {
			out = append(out, tagEffect{int(v.Get("TagID").Int()), v.Get("Effect").Float()})
			return true
		})
		return out
	}

	recipesTagsEffects := readTagEffects("RecipesTagsEffect")
	chefsTagsEffects := readTagEffects("ChefsTagsEffect")

	type idEffect struct{ id int; effect float64 }
	var recipesEffects []idEffect
	gr.Get("RecipesEffect").ForEach(func(_, v gjson.Result) bool {
		recipesEffects = append(recipesEffects, idEffect{int(v.Get("RecipeID").Int()), v.Get("Effect").Float()})
		return true
	})
	var recipesSkillsEffects []skillEffect
	gr.Get("RecipesSkillsEffect").ForEach(func(_, v gjson.Result) bool {
		recipesSkillsEffects = append(recipesSkillsEffects, skillEffect{parseCookSkill(v.Get("Skill").String()), v.Get("Effect").Float()})
		return true
	})
	enableChefTags := readIntSliceFromArray(gr.Get("EnableChefTags"))

	var recipes []Recipe
	for idx := range gc.recipes {
		pr := gc.recipes[idx] // copy
		if hasCookbookLimit && pr.Rarity > cookbookRarityLimit {
			continue
		}

		for _, tag := range pr.Tags {
			for _, rte := range recipesTagsEffects {
				if tag == rte.tagID {
					pr.Addition = toFixed2(pr.Addition + rte.effect)
					break
				}
			}
		}
		for _, re := range recipesEffects {
			if pr.RecipeID == re.id {
				pr.Addition = toFixed2(pr.Addition + re.effect)
			}
		}
		for _, rse := range recipesSkillsEffects {
			if recipeHasSkill(&pr.Recipe, rse.skill) {
				pr.Addition = toFixed2(pr.Addition + rse.effect)
			}
		}

		setIRecipeData(&pr, calGlobal, exRecipeSet[pr.RecipeID], &rule)

		qty := getIRecipeQty(&pr, mats, &rule)
		if qty == 0 {
			continue
		}

		recipes = append(recipes, pr.Recipe)
	}
	rule.Recipes = recipes

	// Chefs
	equipMap := gc.equipByID

	var outChefs []Chef
	for idx := range gc.chefs {
		pc := &gc.chefs[idx]
		if hasChefLimit && pc.Rarity > chefRarityLimit {
			continue
		}
		if len(enableChefTags) > 0 {
			found := false
			for _, ect := range enableChefTags {
				for _, ct := range pc.Tags {
					if ct == ect {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		addition := pc.Addition
		for _, tag := range pc.Tags {
			for _, cte := range chefsTagsEffects {
				if tag == cte.tagID {
					addition = toFixed2(addition + cte.effect)
					break
				}
			}
		}

		chef := toChef(pc, addition)

		var equipObj *Equip
		if chef.EquipID != 0 {
			eq := equipMap[chef.EquipID]
			equipObj = &Equip{EquipID: eq.equipID, Effect: eq.effects}
		}
		chef.EquipEffect = equipObj

		applyChefData(&chef, equipObj, calGlobal, nil, calSelf, &rule, gc.ultimateData.qixia)
		outChefs = append(outChefs, chef)
	}
	rule.Chefs = outChefs

	return rule
}

func setIRecipeData(r *iRecipe, globalUlt []SkillEffect, useEx bool, rule *Rule) {
	r.LimitVal = r.Limit
	r.UltimateAddition = 0

	if !rule.DisableChefSkillEffect {
		for _, g := range globalUlt {
			if g.Type == EffMaxEquipLimit && g.Rarity == r.Rarity {
				r.LimitVal += int(g.Value)
			}
		}
		r.UltimateAddition = computeIRecipePriceAddition(globalUlt, &r.Recipe)
	}

	r.Price = r.BasePrice
	if useEx {
		r.Price += r.ExPrice
	}
}

func computeRecipeMasks(r *Recipe) {
	var price, basic uint64
	var hasFish, hasCreation, hasMeat, hasVegetable bool
	for i := range r.Materials {
		switch r.Materials[i].Origin {
		case OriginPond:
			hasFish = true
		case OriginWorkshop:
			hasCreation = true
		case OriginPasture, OriginCoop, OriginPigsty:
			hasMeat = true
		case OriginShed, OriginField, OriginForest:
			hasVegetable = true
		}
	}
	if hasFish {
		price |= 1 << EffUseFish
		basic |= 1 << EffBasicPriceUseFish
	}
	if hasCreation {
		price |= 1 << EffUseCreation
		basic |= 1 << EffBasicPriceUseCreation
	}
	if hasMeat {
		price |= 1 << EffUseMeat
		basic |= 1 << EffBasicPriceUseMeat
	}
	if hasVegetable {
		price |= 1 << EffUseVegetable
		basic |= 1 << EffBasicPriceUseVegetable
	}
	if r.Stirfry > 0 {
		price |= 1 << EffUseStirfry
		basic |= 1 << EffBasicPriceUseStirfry
	}
	if r.Boil > 0 {
		price |= 1 << EffUseBoil
		basic |= 1 << EffBasicPriceUseBoil
	}
	if r.Fry > 0 {
		price |= 1 << EffUseFry
		basic |= 1 << EffBasicPriceUseFry
	}
	if r.Knife > 0 {
		price |= 1 << EffUseKnife
		basic |= 1 << EffBasicPriceUseKnife
	}
	if r.Bake > 0 {
		price |= 1 << EffUseBake
		basic |= 1 << EffBasicPriceUseBake
	}
	if r.Steam > 0 {
		price |= 1 << EffUseSteam
		basic |= 1 << EffBasicPriceUseSteam
	}
	switch r.Condiment {
	case CondimentSweet:
		price |= 1 << EffUseSweet
		basic |= 1 << EffBasicPriceUseSweet
	case CondimentSour:
		price |= 1 << EffUseSour
		basic |= 1 << EffBasicPriceUseSour
	case CondimentSpicy:
		price |= 1 << EffUseSpicy
		basic |= 1 << EffBasicPriceUseSpicy
	case CondimentSalty:
		price |= 1 << EffUseSalty
		basic |= 1 << EffBasicPriceUseSalty
	case CondimentBitter:
		price |= 1 << EffUseBitter
		basic |= 1 << EffBasicPriceUseBitter
	case CondimentTasty:
		price |= 1 << EffUseTasty
		basic |= 1 << EffBasicPriceUseTasty
	}
	price |= 1 << EffCookbookPrice
	basic |= 1 << EffBasicPrice
	r.PriceMask = price
	r.BasicMask = basic
}

func computeIRecipePriceAddition(effects []SkillEffect, r *Recipe) float64 {
	var price float64
	for _, eff := range effects {
		if eff.ConditionType != CondTypeNone {
			continue
		}
		if r.PriceMask&(1<<eff.Type) != 0 || isRecipePriceSpecial(&eff, r, nil) {
			price += eff.Value
		}
	}
	return price
}

func getIRecipeQty(r *iRecipe, mats []Material, rule *Rule) int {
	maxQty := 1
	if !rule.DisableMultiCookbook {
		maxQty = r.LimitVal
	}
	for _, rm := range r.Materials {
		found := false
		for _, m := range mats {
			if rm.Material == m.MaterialID {
				found = true
				if m.Quantity > 0 {
					qty := m.Quantity / rm.Quantity
					if qty < maxQty {
						maxQty = qty
					}
				}
				break
			}
		}
		if !found {
			return 0
		}
	}
	if maxQty < 0 {
		return 0
	}
	return maxQty
}

func toChef(pc *iChef, addition float64) Chef {
	c := Chef{
		ChefID:              pc.ChefID,
		Name:                pc.Name,
		Rarity:              pc.Rarity,
		Addition:            addition,
		Tags:                pc.Tags,
		SpecialSkillEffect:  pc.SpecialSkillEffect,
		UltimateSkillEffect: pc.UltimateSkillEffect,
		EquipID:             pc.EquipID,
		Stirfry:             pc.Stirfry,
		Boil:                pc.Boil,
		Knife:               pc.Knife,
		Fry:                 pc.Fry,
		Bake:                pc.Bake,
		Steam:               pc.Steam,
		Disk:                pc.Disk,
		Got:                 pc.Got,
	}
	c.TagSet = intsToBoolSet(pc.Tags)
	return c
}

// intsToBoolSet builds a []bool indexed by value for O(1) membership checks.
func intsToBoolSet(ids []int) []bool {
	if len(ids) == 0 {
		return nil
	}
	max := 0
	for _, v := range ids {
		if v > max {
			max = v
		}
	}
	set := make([]bool, max+1)
	for _, v := range ids {
		set[v] = true
	}
	return set
}

func buildCalGlobalUltimate(global []SkillEffect) []SkillEffect {
	skillTypes := [6]EffType{EffStirfry, EffBoil, EffKnife, EffFry, EffBake, EffSteam}

	var skillVals [6]float64
	var maleVal, femaleVal float64
	limitVals := make(map[int]float64)
	useAllVals := make(map[int]float64)

	for _, g := range global {
		switch {
		case g.Tag == 1:
			maleVal = g.Value
		case g.Tag == 2:
			femaleVal = g.Value
		default:
			switch g.Type {
			case EffStirfry:
				skillVals[0] = g.Value
			case EffBoil:
				skillVals[1] = g.Value
			case EffKnife:
				skillVals[2] = g.Value
			case EffFry:
				skillVals[3] = g.Value
			case EffBake:
				skillVals[4] = g.Value
			case EffSteam:
				skillVals[5] = g.Value
			case EffMaxEquipLimit:
				limitVals[g.Rarity] = g.Value
			case EffUseAll:
				useAllVals[g.Rarity] = g.Value
			}
		}
	}

	var result []SkillEffect
	for i, st := range skillTypes {
		if skillVals[i] != 0 {
			result = append(result, SkillEffect{Type: st, Value: skillVals[i], Condition: CondScopeGlobal, Cal: CalAbs})
		}
	}
	for _, gv := range [2]struct{ tag int; val float64 }{{1, maleVal}, {2, femaleVal}} {
		if gv.val != 0 {
			for _, st := range skillTypes {
				result = append(result, SkillEffect{Type: st, Value: gv.val, Condition: CondScopeGlobal, Cal: CalAbs, Tag: gv.tag})
			}
		}
	}
	for r := 1; r <= 5; r++ {
		if v, ok := limitVals[r]; ok && v != 0 {
			result = append(result, SkillEffect{Type: EffMaxEquipLimit, Value: v, Condition: CondScopeGlobal, Cal: CalAbs, Rarity: r})
		}
	}
	for r := 1; r <= 5; r++ {
		if v, ok := useAllVals[r]; ok && v != 0 {
			result = append(result, SkillEffect{Type: EffUseAll, Value: v, Condition: CondScopeGlobal, Cal: CalPercent, Rarity: r})
		}
	}
	return result
}

func buildCalSelfUltimate(gc *gameCache, selfChefIDs []int) []SelfUltimateEntry {
	idSet := make(map[int]bool, len(selfChefIDs))
	for _, id := range selfChefIDs {
		idSet[id] = true
	}
	var result []SelfUltimateEntry
	for i := range gc.chefs {
		c := &gc.chefs[i]
		if !idSet[c.ChefID] {
			continue
		}
		var selfEffects []SkillEffect
		for _, eff := range c.UltimateSkillEffect {
			if eff.Condition == CondScopeSelf {
				selfEffects = append(selfEffects, eff)
			}
		}
		if len(selfEffects) > 0 {
			result = append(result, SelfUltimateEntry{ChefID: c.ChefID, Effect: selfEffects})
		}
	}
	return result
}

func parseIntentArray(dataJSON, key string) []Intent {
	var out []Intent
	gjson.Get(dataJSON, key).ForEach(func(_, v gjson.Result) bool {
		cvResult := v.Get("conditionValue")
		condType := parseIntentCondType(v.Get("conditionType").String())
		intent := Intent{
			IntentID:      int(v.Get("intentId").Int()),
			BuffID:        int(v.Get("buffId").Int()),
			EffectType:    parseIntentEffType(v.Get("effectType").String()),
			EffectValue:   v.Get("effectValue").Float(),
			ConditionType: condType,
			LastRounds:    int(v.Get("lastRounds").Int()),
		}
		intent.ConditionValueInt = int(cvResult.Int())
		if cvResult.Type == gjson.String {
			switch condType {
			case IntCondCookSkill, IntCondGroup:
				intent.CondValSkill = parseCookSkill(cvResult.String())
			case IntCondCondimentSkill:
				intent.CondValCondiment = parseCondimentType(cvResult.String())
			}
		}
		out = append(out, intent)
		return true
	})
	return out
}

func readIntFloatMap(v gjson.Result) map[int]float64 {
	if !v.Exists() || !v.IsObject() {
		return nil
	}
	m := make(map[int]float64)
	v.ForEach(func(key, val gjson.Result) bool {
		m[int(key.Int())] = val.Float()
		return true
	})
	return m
}

func toBool(v gjson.Result) bool {
	switch v.Type {
	case gjson.True:
		return true
	case gjson.Number:
		return v.Float() != 0
	}
	return false
}

func readIntSliceFromArray(v gjson.Result) []int {
	arr := v.Array()
	out := make([]int, len(arr))
	for i, item := range arr {
		out[i] = int(item.Int())
	}
	return out
}
