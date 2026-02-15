package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"time"
)

// BenchOutput is the JSON-serializable result of a full benchmark run.
type BenchOutput struct {
	Date    string          `json:"date"`
	Workers int             `json:"workers"`
	Results []ContestResult `json:"results"`
	TotalMs int64           `json:"totalMs"`
}

// cfg is set once in main() and threaded through all calls.
var cfg Config

func runAll(input *InputData, jsonOut bool) {
	gd := input.ToGameData()
	var results []ContestResult
	var totalMs int64

	for i := range input.Contests {
		c := &input.Contests[i]
		fmt.Fprintf(os.Stderr, "[%d/%d] %s ...\n", i+1, len(input.Contests), c.Name)
		r, bestState := runContest(c, gd, cfg)
		results = append(results, r)
		totalMs += r.TimeMs
		fmt.Fprintf(os.Stderr, "  %s: %d in %.1fs\n", c.Name, r.Score, float64(r.TimeMs)/1000)
		if bestState != nil {
			fmt.Println(FormatResult(c.Rules, bestState, gd, c))
		}
	}

	if jsonOut {
		out := BenchOutput{
			Date:    time.Now().UTC().Format(time.RFC3339),
			Workers: runtime.NumCPU(),
			Results: results,
			TotalMs: totalMs,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
	} else {
		printTable(results, totalMs)
	}
}

func printTable(results []ContestResult, totalMs int64) {
	fmt.Printf("%-24s %10s %8s\n", "Contest", "Score", "Time")
	fmt.Printf("%-24s %10s %8s\n", "------------------------", "----------", "--------")
	totalScore := 0
	for _, r := range results {
		totalScore += r.Score
		fmt.Printf("%-24s %10d %7.1fs\n", r.Name, r.Score, float64(r.TimeMs)/1000)
	}
	fmt.Printf("%-24s %10s %8s\n", "------------------------", "----------", "--------")
	fmt.Printf("%-24s %10d %7.1fs\n", "TOTAL", totalScore, float64(totalMs)/1000)
}

func runSingle(input *InputData, ruleID int, jsonOut bool) {
	contest := FindContest(input, ruleID)
	if contest == nil {
		fmt.Fprintf(os.Stderr, "contest ruleId %d not found\n", ruleID)
		os.Exit(1)
	}

	gd := input.ToGameData()
	r, bestState := runContest(contest, gd, cfg)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	} else {
		fmt.Printf("Contest: %s (ruleId=%d)\n", r.Name, r.RuleID)
		fmt.Printf("Total: %d in %.1fs\n", r.Score, float64(r.TimeMs)/1000)
	}
	if bestState != nil {
		fmt.Println(FormatResult(contest.Rules, bestState, gd, contest))
	}
}

const usage = `Usage: banquet-optimizer <data.min.json> <archive.json> [ruleId]

Positional arguments:
  data.min.json   Path to raw game data
  archive.json    Path to player archive
  ruleId          Contest rule ID (0 or omitted = run all)

Flags:
`

func main() {
	jsonOut := flag.Bool("json", false, "Output results as JSON")
	verbose := flag.Bool("verbose", false, "Print detailed search progress to stderr")
	memprof := flag.String("memprof", "", "write memory profile to file")
	cpuprof := flag.String("cpuprof", "", "write CPU profile to file")

	// Search tuning overrides
	seeds := flag.Int("seeds", 0, "MaxDiverseSeeds override (default 12)")
	rounds := flag.Int("rounds", 0, "MaxRounds override (default 5)")
	refine := flag.Int("refine", 0, "RefineIter override (default 5)")
	recipek := flag.Int("recipek", 0, "RecipeSeedK override (default 5)")
	chefk := flag.Int("chefk", 0, "ChefPerSeed override (default 3)")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg = DefaultConfig()
	cfg.Verbose = *verbose
	if *seeds > 0 {
		cfg.MaxDiverseSeeds = *seeds
	}
	if *rounds > 0 {
		cfg.MaxRounds = *rounds
	}
	if *refine > 0 {
		cfg.RefineIter = *refine
	}
	if *recipek > 0 {
		cfg.RecipeSeedK = *recipek
	}
	if *chefk > 0 {
		cfg.ChefPerSeed = *chefk
	}

	if *cpuprof != "" {
		f, err := os.Create(*cpuprof)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating cpu profile: %v\n", err)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
	}

	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	rawDataPath := args[0]
	archivePath := args[1]

	var ruleID int
	if len(args) >= 3 {
		var err error
		ruleID, err = strconv.Atoi(args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid ruleId %q\n", args[2])
			os.Exit(1)
		}
	}

	input, err := LoadRawData(rawDataPath, archivePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d contests, %d intents, %d buffs\n",
		len(input.Contests), len(input.Global.Intents), len(input.Global.Buffs))

	if ruleID == 0 {
		runAll(input, *jsonOut)
	} else {
		runSingle(input, ruleID, *jsonOut)
	}

	if *memprof != "" {
		f, err := os.Create(*memprof)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating memory profile: %v\n", err)
			os.Exit(1)
		}
		runtime.GC()
		pprof.WriteHeapProfile(f)
		f.Close()
	}
}
