package main

// ── Core data types matching the JS scoring pipeline ──

// Addition holds absolute and percentage modifiers applied to a base value.
type Addition struct {
	Abs     float64
	Percent float64
}

// SkillEffect represents a single skill/buff effect entry.
type SkillEffect struct {
	Type               string        `json:"type"`
	Cal                string        `json:"cal"`    // "Abs" or "Percent"
	Value              float64       `json:"value"`
	Rarity             int           `json:"rarity"`
	Condition          string        `json:"condition"` // "Self", "Partial", "Next"
	ConditionType      string        `json:"conditionType"`
	ConditionValue     interface{}   `json:"conditionValue"`
	ConditionValueList []interface{} `json:"conditionValueList"`
	Tag                int           `json:"tag"` // chef tag filter
}

// LimitEffect represents a rarity-gated limit bonus (e.g. extra cookbook copies).
type LimitEffect struct {
	Rarity int `json:"rarity"`
	Value  int `json:"value"`
}

// MaterialReduce describes a chef's ability to reduce material consumption.
type MaterialReduce struct {
	ConditionType      string  `json:"conditionType"`
	ConditionValueList []int   `json:"conditionValueList"`
	Cal                string  `json:"cal"`
	Value              float64 `json:"value"`
}

// Amber is a gem socket on a chef's disk that provides skill effects.
type Amber struct {
	Data *AmberData `json:"data"`
}

// AmberData holds the per-level skill effects for an amber.
type AmberData struct {
	AllEffect [][]SkillEffect `json:"allEffect"` // allEffect[level-1]
}

// Disk is a chef's equipment disk containing amber sockets.
type Disk struct {
	Level    int     `json:"level"`
	MaxLevel int     `json:"maxLevel"`
	Ambers   []Amber `json:"ambers"`
}

// Chef represents a chef with base stats, skills, equipment, and computed skill values.
type Chef struct {
	ChefID              int              `json:"chefId"`
	Name                string           `json:"name"`
	Rarity              int              `json:"rarity"`
	Addition            float64          `json:"addition"`
	Tags                []int            `json:"tags"`
	SpecialSkillEffect  []SkillEffect    `json:"specialSkillEffect"`
	SelfUltimateEffect  []SkillEffect    `json:"selfUltimateEffect"`  // computed by ApplyChefData
	UltimateSkillEffect []SkillEffect    `json:"ultimateSkillEffect"`
	MaxLimitEffect      []LimitEffect    `json:"maxLimitEffect"`      // computed by ApplyChefData
	MaterialEffects     []MaterialReduce `json:"materialEffects"`     // computed by ApplyChefData
	Disk                Disk             `json:"disk"`
	EquipID     int    `json:"equipId"`
	EquipEffect *Equip `json:"equipEffect"` // embedded equip from preprocessor

	// Base skill values
	Stirfry int `json:"stirfry"`
	Boil    int `json:"boil"`
	Knife   int `json:"knife"`
	Fry     int `json:"fry"`
	Bake    int `json:"bake"`
	Steam   int `json:"steam"`

	// Computed skill values (set by ApplyChefData)
	StirfryVal float64 `json:"stirfryVal"`
	BoilVal    float64 `json:"boilVal"`
	KnifeVal   float64 `json:"knifeVal"`
	FryVal     float64 `json:"fryVal"`
	BakeVal    float64 `json:"bakeVal"`
	SteamVal   float64 `json:"steamVal"`

	Got bool `json:"got"`
}

// RecipeMaterial is a single ingredient entry in a recipe.
type RecipeMaterial struct {
	Material int    `json:"material"`
	Quantity int    `json:"quantity"`
	Origin   string `json:"origin"`
}

// Recipe holds a dish's base stats, price, skill requirements, and computed bonuses.
type Recipe struct {
	RecipeID         int              `json:"recipeId"`
	Name             string           `json:"name"`
	Rarity           int              `json:"rarity"`
	Price            float64          `json:"price"`
	Stirfry          int              `json:"stirfry"`
	Boil             int              `json:"boil"`
	Knife            int              `json:"knife"`
	Fry              int              `json:"fry"`
	Bake             int              `json:"bake"`
	Steam            int              `json:"steam"`
	Materials        []RecipeMaterial `json:"materials"`
	Addition         float64          `json:"addition"`
	ActivityAddition float64          `json:"activityAddition"`
	UltimateAddition float64          `json:"ultimateAddition"`
	LimitVal         int              `json:"limitVal"`
	Condiment        string           `json:"condiment"`
	Tags             []int            `json:"tags"`
	Got              bool             `json:"got"`
}

// Material represents an ingredient with its available quantity and score addition.
type Material struct {
	MaterialID int     `json:"materialId"`
	Quantity   int     `json:"quantity"`
	Addition   float64 `json:"addition"`
	Name       string  `json:"name"`
	Origin     string  `json:"origin"`
}

// Equip represents a piece of equipment with skill effects.
type Equip struct {
	EquipID int           `json:"equipId"`
	Effect  []SkillEffect `json:"effect"`
}

// SelfUltimateEntry pairs a chef ID with their self-targeted ultimate skill effects.
type SelfUltimateEntry struct {
	ChefID int           `json:"chefId"`
	Effect []SkillEffect `json:"effect"`
}

// QixiaEntry holds the SwordsUnited (qixia) skill bonuses keyed by chef tag.
type QixiaEntry struct {
	Stirfry float64 `json:"Stirfry"`
	Boil    float64 `json:"Boil"`
	Knife   float64 `json:"Knife"`
	Fry     float64 `json:"Fry"`
	Bake    float64 `json:"Bake"`
	Steam   float64 `json:"Steam"`
}

// Rule represents a single guest/contest rule.
type Rule struct {
	Title                   string             `json:"Title"`
	Satiety                 int                `json:"Satiety"`
	IntentList              [][]int            `json:"IntentList"`
	ScoreMultiply           float64            `json:"scoreMultiply"`
	ScorePow                float64            `json:"scorePow"`
	ScoreAdd                int                `json:"scoreAdd"`
	IsActivity              bool               `json:"IsActivity"`
	DisableMultiCookbook    bool               `json:"DisableMultiCookbook"`
	DisableCookbookRank     bool               `json:"DisableCookbookRank"`
	DisableChefSkillEffect  bool               `json:"DisableChefSkillEffect"`
	DisableEquipSkillEffect bool               `json:"DisableEquipSkillEffect"`
	DisableCondimentEffect  bool               `json:"DisableCondimentEffect"`
	DisableDecorationEffect bool               `json:"DisableDecorationEffect"`
	MaterialsEffect         bool               `json:"MaterialsEffect"`
	RecipeEffect            map[int]float64    `json:"RecipeEffect"`
	ChefTagEffect           map[int]float64    `json:"ChefTagEffect"`
	Materials               []Material         `json:"materials"`
	DecorationEffect        float64            `json:"decorationEffect"`
	SatisfyRewardType       int                `json:"SatisfyRewardType"`
	SatisfyExtraValue       float64            `json:"SatisfyExtraValue"`
	SatisfyDeductValue      float64            `json:"SatisfyDeductValue"`
	CalPartialChefIDs       []int              `json:"calPartialChefIds"`

	CalGlobalUltimateData   []SkillEffect       `json:"calGlobalUltimateData"`
	CalSelfUltimateData     []SelfUltimateEntry  `json:"calSelfUltimateData"`
	CalActivityUltimateData []SkillEffect        `json:"calActivityUltimateData"`
	CalQixiaData            map[int]*QixiaEntry  `json:"calQixiaData"`

	Chefs   []Chef  `json:"chefs"`
	Equips  []Equip `json:"equips"`
	Recipes []Recipe `json:"recipes"`

	GlobalBuffList []int `json:"GlobalBuffList"`
}

// ── Intent / Buff ──

// Intent represents a guest intent or buff that modifies recipe scoring.
type Intent struct {
	IntentID       int         `json:"intentId"`
	EffectType     string      `json:"effectType"`
	EffectValue    float64     `json:"effectValue"`
	ConditionType  string      `json:"conditionType"`
	ConditionValue interface{} `json:"conditionValue"`
	BuffID         int         `json:"buffId"`
	LastRounds     int         `json:"lastRounds"`
}

// GameData holds the global intent and buff definitions used during scoring.
type GameData struct {
	Intents []Intent `json:"intents"`
	Buffs   []Intent `json:"buffs"`
}

// ── Simulation state (index-based, used by search layer) ──

// SlotState tracks the chef and recipe assignments for one position in a guest rule.
type SlotState struct {
	ChefIdx    int      // index into Rule.Chefs (-1 = empty)
	RecipeIdxs [3]int   // indices into Rule.Recipes (-1 = empty)
	Quantities [3]int
	MaxQty     [3]int
}

// RuleState is the set of slot assignments for one guest rule.
type RuleState []SlotState

// SimState is the complete assignment state across all guest rules in a contest.
type SimState []RuleState

// ── Scoring intermediates (internal to score.go) ──

// PartialAdd pairs a skill effect with the number of times it applies.
type PartialAdd struct {
	Effect *SkillEffect
	Count  int
}

// recipeSlot is the object-based recipe slot used internally during scoring.
type recipeSlot struct {
	Data     *Recipe
	Quantity int
	Max      int
	Satiety  int
}

// customEntry is the object-based per-chef-position entry used internally during scoring.
type customEntry struct {
	Chef    *Chef
	Equip   *Equip
	Recipes *[3]recipeSlot
}
