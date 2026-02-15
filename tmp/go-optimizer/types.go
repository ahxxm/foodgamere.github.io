package main

type EffType int

const (
	EffNone EffType = iota
	// Cooking skill types
	EffStirfry
	EffBoil
	EffKnife
	EffFry
	EffBake
	EffSteam
	// Price addition types (Use*)
	EffUseAll
	EffUseFish
	EffUseCreation
	EffUseMeat
	EffUseVegetable
	EffUseStirfry
	EffUseBoil
	EffUseFry
	EffUseKnife
	EffUseBake
	EffUseSteam
	EffUseSweet
	EffUseSour
	EffUseSpicy
	EffUseSalty
	EffUseBitter
	EffUseTasty
	EffGoldGain
	EffCookbookPrice
	// Basic price addition types (contiguous for isBasicPriceUseType range check)
	EffBasicPrice
	EffBasicPriceUseFish
	EffBasicPriceUseCreation
	EffBasicPriceUseMeat
	EffBasicPriceUseVegetable
	EffBasicPriceUseStirfry
	EffBasicPriceUseBoil
	EffBasicPriceUseFry
	EffBasicPriceUseKnife
	EffBasicPriceUseBake
	EffBasicPriceUseSteam
	EffBasicPriceUseSweet
	EffBasicPriceUseSour
	EffBasicPriceUseSpicy
	EffBasicPriceUseSalty
	EffBasicPriceUseBitter
	EffBasicPriceUseTasty
	// Chef data types
	EffMaxEquipLimit
	EffMaterialReduce
	EffMutiEquipmentSkill
)

func isBasicPriceUseType(t EffType) bool {
	return t >= EffBasicPriceUseFish && t <= EffBasicPriceUseTasty
}

type CalType int

const (
	CalNone CalType = iota
	CalAbs
	CalPercent
)

type CondScope int

const (
	CondScopeNone CondScope = iota
	CondScopeSelf
	CondScopePartial
	CondScopeNext
	CondScopeGlobal
)

type CondType int

const (
	CondTypeNone CondType = iota
	CondTypeRank
	CondTypePerRank
	CondTypeExcessCookbookNum
	CondTypeFewerCookbookNum
	CondTypeCookbookRarity
	CondTypeChefTag
	CondTypeCookbookTag
	CondTypeSameSkill
	CondTypePerSkill
	CondTypeMaterialReduce
	CondTypeSwordsUnited
)

type IntentCondType int

const (
	IntCondNone IntentCondType = iota
	IntCondGroup
	IntCondChefStar
	IntCondRank
	IntCondCondimentSkill
	IntCondCookSkill
	IntCondRarity
	IntCondOrder
)

type IntentEffType int

const (
	IntEffNone IntentEffType = iota
	IntEffCreateBuff
	IntEffCreateIntent
	IntEffIntentAdd
	IntEffBasicPriceChange
	IntEffBasicPriceChangePercent
	IntEffSatietyChange
	IntEffSatietyChangePercent
	IntEffSetSatietyValue
	IntEffPriceChangePercent
)

type CookSkill int

const (
	CookNone CookSkill = iota
	CookStirfry
	CookBoil
	CookKnife
	CookFry
	CookBake
	CookSteam
)

type CondimentType int

const (
	CondimentNone CondimentType = iota
	CondimentSweet
	CondimentSour
	CondimentSpicy
	CondimentSalty
	CondimentBitter
	CondimentTasty
)

type OriginType int

const (
	OriginNone OriginType = iota
	OriginPond     // 池塘
	OriginWorkshop // 作坊
	OriginPasture  // 牧场
	OriginCoop     // 鸡舍
	OriginPigsty   // 猪圈
	OriginShed     // 菜棚
	OriginField    // 菜地
	OriginForest   // 森林
)

func parseEffType(s string) EffType {
	switch s {
	case "Stirfry":
		return EffStirfry
	case "Boil":
		return EffBoil
	case "Knife":
		return EffKnife
	case "Fry":
		return EffFry
	case "Bake":
		return EffBake
	case "Steam":
		return EffSteam
	case "UseAll":
		return EffUseAll
	case "UseFish":
		return EffUseFish
	case "UseCreation":
		return EffUseCreation
	case "UseMeat":
		return EffUseMeat
	case "UseVegetable":
		return EffUseVegetable
	case "UseStirfry":
		return EffUseStirfry
	case "UseBoil":
		return EffUseBoil
	case "UseFry":
		return EffUseFry
	case "UseKnife":
		return EffUseKnife
	case "UseBake":
		return EffUseBake
	case "UseSteam":
		return EffUseSteam
	case "UseSweet":
		return EffUseSweet
	case "UseSour":
		return EffUseSour
	case "UseSpicy":
		return EffUseSpicy
	case "UseSalty":
		return EffUseSalty
	case "UseBitter":
		return EffUseBitter
	case "UseTasty":
		return EffUseTasty
	case "Gold_Gain":
		return EffGoldGain
	case "CookbookPrice":
		return EffCookbookPrice
	case "BasicPrice":
		return EffBasicPrice
	case "BasicPriceUseFish":
		return EffBasicPriceUseFish
	case "BasicPriceUseCreation":
		return EffBasicPriceUseCreation
	case "BasicPriceUseMeat":
		return EffBasicPriceUseMeat
	case "BasicPriceUseVegetable":
		return EffBasicPriceUseVegetable
	case "BasicPriceUseStirfry":
		return EffBasicPriceUseStirfry
	case "BasicPriceUseBoil":
		return EffBasicPriceUseBoil
	case "BasicPriceUseFry":
		return EffBasicPriceUseFry
	case "BasicPriceUseKnife":
		return EffBasicPriceUseKnife
	case "BasicPriceUseBake":
		return EffBasicPriceUseBake
	case "BasicPriceUseSteam":
		return EffBasicPriceUseSteam
	case "BasicPriceUseSweet":
		return EffBasicPriceUseSweet
	case "BasicPriceUseSour":
		return EffBasicPriceUseSour
	case "BasicPriceUseSpicy":
		return EffBasicPriceUseSpicy
	case "BasicPriceUseSalty":
		return EffBasicPriceUseSalty
	case "BasicPriceUseBitter":
		return EffBasicPriceUseBitter
	case "BasicPriceUseTasty":
		return EffBasicPriceUseTasty
	case "MaxEquipLimit":
		return EffMaxEquipLimit
	case "MaterialReduce":
		return EffMaterialReduce
	case "MutiEquipmentSkill":
		return EffMutiEquipmentSkill
	}
	return EffNone
}

func parseCalType(s string) CalType {
	switch s {
	case "Abs":
		return CalAbs
	case "Percent":
		return CalPercent
	}
	return CalNone
}

func parseCondScope(s string) CondScope {
	switch s {
	case "Self":
		return CondScopeSelf
	case "Partial":
		return CondScopePartial
	case "Next":
		return CondScopeNext
	case "Global":
		return CondScopeGlobal
	}
	return CondScopeNone
}

func parseCondType(s string) CondType {
	switch s {
	case "Rank":
		return CondTypeRank
	case "PerRank":
		return CondTypePerRank
	case "ExcessCookbookNum":
		return CondTypeExcessCookbookNum
	case "FewerCookbookNum":
		return CondTypeFewerCookbookNum
	case "CookbookRarity":
		return CondTypeCookbookRarity
	case "ChefTag":
		return CondTypeChefTag
	case "CookbookTag":
		return CondTypeCookbookTag
	case "SameSkill":
		return CondTypeSameSkill
	case "PerSkill":
		return CondTypePerSkill
	case "MaterialReduce":
		return CondTypeMaterialReduce
	case "SwordsUnited":
		return CondTypeSwordsUnited
	}
	return CondTypeNone
}

func parseIntentCondType(s string) IntentCondType {
	switch s {
	case "Group":
		return IntCondGroup
	case "ChefStar":
		return IntCondChefStar
	case "Rank":
		return IntCondRank
	case "CondimentSkill":
		return IntCondCondimentSkill
	case "CookSkill":
		return IntCondCookSkill
	case "Rarity":
		return IntCondRarity
	case "Order":
		return IntCondOrder
	}
	return IntCondNone
}

func parseIntentEffType(s string) IntentEffType {
	switch s {
	case "CreateBuff":
		return IntEffCreateBuff
	case "CreateIntent":
		return IntEffCreateIntent
	case "IntentAdd":
		return IntEffIntentAdd
	case "BasicPriceChange":
		return IntEffBasicPriceChange
	case "BasicPriceChangePercent":
		return IntEffBasicPriceChangePercent
	case "SatietyChange":
		return IntEffSatietyChange
	case "SatietyChangePercent":
		return IntEffSatietyChangePercent
	case "SetSatietyValue":
		return IntEffSetSatietyValue
	case "PriceChangePercent":
		return IntEffPriceChangePercent
	}
	return IntEffNone
}

func parseCookSkill(s string) CookSkill {
	switch s {
	case "stirfry", "Stirfry":
		return CookStirfry
	case "boil", "Boil":
		return CookBoil
	case "knife", "Knife":
		return CookKnife
	case "fry", "Fry":
		return CookFry
	case "bake", "Bake":
		return CookBake
	case "steam", "Steam":
		return CookSteam
	}
	return CookNone
}

func parseCondimentType(s string) CondimentType {
	switch s {
	case "Sweet":
		return CondimentSweet
	case "Sour":
		return CondimentSour
	case "Spicy":
		return CondimentSpicy
	case "Salty":
		return CondimentSalty
	case "Bitter":
		return CondimentBitter
	case "Tasty":
		return CondimentTasty
	}
	return CondimentNone
}

func parseOriginType(s string) OriginType {
	switch s {
	case "池塘":
		return OriginPond
	case "作坊":
		return OriginWorkshop
	case "牧场":
		return OriginPasture
	case "鸡舍":
		return OriginCoop
	case "猪圈":
		return OriginPigsty
	case "菜棚":
		return OriginShed
	case "菜地":
		return OriginField
	case "森林":
		return OriginForest
	}
	return OriginNone
}

type Addition struct {
	Abs     float64
	Percent float64
}

type SkillEffect struct {
	Type               EffType
	Cal                CalType
	Value              float64
	Rarity             int
	Condition          CondScope
	ConditionType      CondType
	ConditionValueList []int
	Tag                int
	ConditionValueInt  int
}

type LimitEffect struct {
	Rarity int
	Value  int
}

type MaterialReduce struct {
	ConditionValueList []int
	Cal                CalType
	Value              float64
}

type Amber struct {
	Data *AmberData
}

type AmberData struct {
	AllEffect [][]SkillEffect // allEffect[level-1]
}

type Disk struct {
	Level    int
	MaxLevel int
	Ambers   []Amber
}

type Chef struct {
	ChefID              int
	Name                string
	Rarity              int
	Addition            float64
	Tags                []int
	TagSet              []bool // indexed by tag ID for O(1) lookup
	SpecialSkillEffect  []SkillEffect
	SelfUltimateEffect  []SkillEffect    // computed by applyChefData
	UltimateSkillEffect []SkillEffect
	MaxLimitEffect      []LimitEffect    // computed by applyChefData
	MaterialEffects     []MaterialReduce // computed by applyChefData
	Disk                Disk
	EquipID             int
	EquipEffect         *Equip // embedded equip from preprocessor

	// Base skill values
	Stirfry int
	Boil    int
	Knife   int
	Fry     int
	Bake    int
	Steam   int

	// Computed skill values (set by applyChefData)
	StirfryVal float64
	BoilVal    float64
	KnifeVal   float64
	FryVal     float64
	BakeVal    float64
	SteamVal   float64

	Got bool
}

type RecipeMaterial struct {
	Material int
	Quantity int
	Origin   OriginType
}

type Recipe struct {
	RecipeID         int
	Name             string
	Rarity           int
	Price            float64
	Stirfry          int
	Boil             int
	Knife            int
	Fry              int
	Bake             int
	Steam            int
	Materials        []RecipeMaterial
	Addition         float64
	ActivityAddition float64
	UltimateAddition float64
	LimitVal         int
	Condiment        CondimentType
	Tags             []int
	TagSet           []bool // indexed by tag ID for O(1) lookup
	Got              bool
	PriceMask        uint64 // bit N set = EffType(N) matches this recipe for price addition
	BasicMask        uint64 // bit N set = EffType(N) matches this recipe for basic addition
}

type Material struct {
	MaterialID int
	Quantity   int
	Addition   float64
	Name       string
	Origin     OriginType
}

type Equip struct {
	EquipID int
	Effect  []SkillEffect
}

type SelfUltimateEntry struct {
	ChefID int
	Effect []SkillEffect
}

type QixiaEntry struct {
	Stirfry float64
	Boil    float64
	Knife   float64
	Fry     float64
	Bake    float64
	Steam   float64
}

type Rule struct {
	Title                   string
	Satiety                 int
	IntentList              [][]int
	ScoreMultiply           float64
	ScorePow                float64
	ScoreAdd                int
	IsActivity              bool
	DisableMultiCookbook    bool
	DisableCookbookRank     bool
	DisableChefSkillEffect  bool
	DisableEquipSkillEffect bool
	DisableDecorationEffect bool
	MaterialsEffect         bool
	RecipeEffect            map[int]float64
	ChefTagEffect           map[int]float64
	Materials               []Material
	DecorationEffect        float64
	SatisfyRewardType       int
	SatisfyExtraValue       float64
	SatisfyDeductValue      float64
	CalPartialChefSet       []bool // indexed by ChefID for O(1) lookup

	CalGlobalUltimateData []SkillEffect
	CalSelfUltimateData   []SelfUltimateEntry
	CalQixiaData          map[int]*QixiaEntry

	Chefs   []Chef
	Recipes []Recipe

	GlobalBuffList []int
}

type Intent struct {
	IntentID         int
	EffectType       IntentEffType
	EffectValue      float64
	ConditionType    IntentCondType
	BuffID           int
	LastRounds       int
	ConditionValueInt int
	CondValSkill     CookSkill     // for CookSkill/Group conditions
	CondValCondiment CondimentType // for CondimentSkill conditions
}

type GameData struct {
	Intents    []Intent
	Buffs      []Intent
	IntentByID []*Intent // indexed by IntentID for O(1) lookup
	BuffByID   []*Intent // indexed by BuffID for O(1) lookup
}

type SlotState struct {
	ChefIdx    int    // index into Rule.Chefs (-1 = empty)
	RecipeIdxs [3]int // indices into Rule.Recipes (-1 = empty)
	Quantities [3]int
	MaxQty     [3]int
}

type RuleState []SlotState

type SimState []RuleState

type PartialAdd struct {
	Effect *SkillEffect
	Count  int
}

type recipeSlot struct {
	Data     *Recipe
	Quantity int
	Max      int
	Satiety  int
}

type customEntry struct {
	Chef    *Chef
	Equip   *Equip
	Recipes *[3]recipeSlot
}
