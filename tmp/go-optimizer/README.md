# Banquet Optimizer (Go)

Standalone binary for food game banquet contest optimization. Reads raw game data directly.

```bash
go run . /path/to/data.min.json /path/to/yx518-archive.json [ruleId]
```

### PGO (Profile-Guided Optimization)

2-pass build: profile first, then rebuild with `-pgo`. Profile is [cross-platform](https://go.dev/doc/pgo#alternatives) (amd64 profile works for arm64 build).

```bash
go build -o banquet-optimizer .
./banquet-optimizer -cpuprof cpu.prof ../../data/data.min.json archive.json
go build -pgo=cpu.prof -ldflags="-s -w" -o banquet-optimizer .
```

## Lambda

Embeds `data.min.json` via `//go:embed`. Accepts POST with `{ruleId, archive}`.

```bash
cp ../../data/data.min.json .
GOOS=linux GOARCH=arm64 go build -tags lambda -pgo=cpu.prof -ldflags="-s -w" -o bootstrap .
zip a.zip bootstrap
rm bootstrap
```

Lambda config:
- Runtime: `provided.al2023`
- Architecture: `arm64`
- Memory: `10240` MB (6 vCPUs)
- Timeout: `300`s
- Function URL: enabled, auth type NONE

Function URL CORS:
- Allow origins: `*`
- Allow methods: `*`
- Allow headers: `content-type`
- Max age: `86400`

API: `POST {"ruleId": 680045, "archive": {"msg": {"chefs": [...], "recipes": [...]}}, "excludeMaterials": [1, 2]}` returns `{"score": 4510981, "timeMs": 4207, "detail": "第1位客人：..."}`. `excludeMaterials` is optional; when present, recipes containing any listed material ID are excluded.

## Contests

| ruleId | Name               |
|--------|--------------------|
| 680005 | 玉贵人+胡喜媚     |
| 680010 | 蓝采和+铁拐李     |
| 680015 | 吕洞宾+曹国舅     |
| 680020 | 韩湘子+钟离权     |
| 680025 | 何仙姑+胡喜媚     |
| 680030 | 韩湘子+蓝采和     |
| 680035 | 打更人+玉贵人     |
| 680040 | 胡喜媚+铁拐李     |
| 680045 | 苏妲己+张果老     |
| 680050 | 王老板+白马王子   |
| 680055 | 钟离权+曹国舅     |
| 680060 | 吕洞宾+胡喜媚     |
| 680065 | 白马王子+吕洞宾   |
| 680070 | 钟离权+玉贵人     |
| 680075 | 胡喜媚+苏妲己     |
| 680080 | 猪猪男孩+御弟哥哥 |
| 680085 | 卷帘大将+白马王子 |
| 680090 | 空空+御弟哥哥     |
| 680095 | 猪猪男孩+空空     |
| 680100 | 卷帘大将+御弟哥哥 |

## Performance

20 contests, unlimited mode, on 8-core WSL2.

| Stage               | Total time | Key change                                                         |
|---------------------|------------|--------------------------------------------------------------------|
| JS baseline         |      3087s | single-threaded, interpreted                                       |
| Go initial          |       425s | compiled, goroutine-parallel deep search                           |
| + intent re-ranking |       473s | phase 2 re-ranks recipes by intent bonus, +310K total score        |
| + parallel refine   |       250s | seed refinement phase also parallel via goroutines                 |
| + buffer reuse      |        96s | pre-allocated scratch buffers, zero alloc in scoring hot path      |
| + indexed lookups   |        77s | map[int]bool -> []bool indexed by ID for chef/recipe/material sets |
| + pointer storage   |        71s | []Intent value slices -> []*Intent, eliminate 120-byte copies      |
| + required matIndex |        60s | material lookup array mandatory, remove linear scan fallback       |
| + more buffer reuse |        57s | partialChefAdds and buildCustomArr also use scratch buffers        |
| + quickselect       |        55s | O(n) top-K selection replacing O(n log n) full sort                |
| + search moves      |        62s | joint climbing, aura/multi-skill seeds, chefCanCook fix, +455K     |
| + int enums         |        57s | string fields → int enums at parse time, eliminate hot-path strcmp  |
| + ranking scratch   |        54s | reuse scratch slices in getRecipeRanking/getChefRanking            |
| + slices.SortFunc   |        48s | sort.Slice (reflection) → slices.SortFunc (generic, no reflection) |
| + micro-opts        |        48s | matIndex 0-sentinel+SIMD clear (0.3ns vs 1450ns/2k), inline min    |
| + PGO build         |        45s | profile-guided optimization, compiler inlines hot-path functions   |
| + recipe bitmask    |        42s | precompute per-recipe uint64 mask, replace 20-case switch in scoring|
