package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

// ContestResult holds the score and timing for a single contest optimization run.
type ContestResult struct {
	Name    string `json:"name"`
	RuleID  int    `json:"ruleId"`
	Score   int    `json:"score"`
	PerRule []int  `json:"perRule,omitempty"`
	TimeMs  int64  `json:"timeMs"`
}

// BenchOutput is the JSON-serializable result of a full benchmark run.
type BenchOutput struct {
	Date    string          `json:"date"`
	Workers int             `json:"workers"`
	Results []ContestResult `json:"results"`
	TotalMs int64           `json:"totalMs"`
}

func runContest(contest *Contest, gd *GameData, targetScore int) (ContestResult, SimState) {
	opt := NewOptimizer(contest, gd)
	score, bestState, elapsed := opt.Optimize(targetScore)

	return ContestResult{
		Name:   contest.Name,
		RuleID: contest.RuleID,
		Score:  score,
		TimeMs: elapsed.Milliseconds(),
	}, bestState
}

func runAll(input *InputData, target int, jsonOut bool) {
	gd := input.ToGameData()
	var results []ContestResult
	var totalMs int64

	for i := range input.Contests {
		c := &input.Contests[i]
		fmt.Fprintf(os.Stderr, "[%d/%d] %s ...\n", i+1, len(input.Contests), c.Name)
		r, bestState := runContest(c, gd, target)
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

func runSingle(input *InputData, ruleID int, target int, jsonOut bool) {
	contest := FindContest(input, ruleID)
	if contest == nil {
		fmt.Fprintf(os.Stderr, "contest ruleId %d not found\n", ruleID)
		os.Exit(1)
	}

	gd := input.ToGameData()
	r, bestState := runContest(contest, gd, target)

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(r)
	} else {
		fmt.Printf("Contest: %s (ruleId=%d)\n", r.Name, r.RuleID)
		for i, s := range r.PerRule {
			fmt.Printf("  Rule %d: %d\n", i+1, s)
		}
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
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	Verbose = *verbose

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
		runAll(input, 0, *jsonOut)
	} else {
		runSingle(input, ruleID, 0, *jsonOut)
	}
}
